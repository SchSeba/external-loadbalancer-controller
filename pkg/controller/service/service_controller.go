/*
Copyright 2018 Sebastian Sch.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/provider"

	"github.com/cloudflare/cfssl/log"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/farm"
	"k8s.io/client-go/tools/record"
)

type ServiceController struct {
	Controller       controller.Controller
	ReconcileService reconcile.Reconciler
}

func NewServiceController(mgr manager.Manager, providerController *provider.ProviderController, farmController *farm.FarmController) (*ServiceController, error) {
	reconcileService := newReconciler(mgr, providerController, farmController)

	controllerInstance, err := newController(mgr, reconcileService)
	if err != nil {
		return nil, err
	}
	serviceController := &ServiceController{Controller: controllerInstance,
		ReconcileService: reconcileService}

	return serviceController, nil

}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, ProviderController *provider.ProviderController, farmController *farm.FarmController) *ReconcileService {
	return &ReconcileService{Client: mgr.GetClient(),
		scheme:             mgr.GetScheme(),
		Event:              mgr.GetRecorder(managerv1alpha1.EventRecorderName),
		ProviderController: ProviderController,
		FarmController:     farmController}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("service-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

var _ reconcile.Reconciler = &ReconcileService{}

// ReconcileService reconciles a Service object
type ReconcileService struct {
	client.Client
	Event              record.EventRecorder
	ProviderController *provider.ProviderController
	FarmController     *farm.FarmController
	scheme             *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;update;patch
func (r *ReconcileService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Service instance
	service := &corev1.Service{}
	err := r.Get(context.TODO(), request.NamespacedName, service)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Infof("%+v", service)

	farmInstance, isCreated, err := r.FarmController.GetOrCreateFarm(service)
	if err != nil {
		log.Errorf("Fail to get or create farm from service %s in namespace %s", service.Name, service.Namespace)
		return reconcile.Result{}, err
	}

	var serviceIpAddress string

	if isCreated {
		serviceIpAddress, err = r.ProviderController.CreateFarm(farmInstance)
	} else if r.FarmController.NeedToUpdate(farmInstance, service) {
		serviceIpAddress, err = r.ProviderController.UpdateFarm(farmInstance)
	}

	if err != nil {
		log.Errorf("Fail to create or update farm on provider for service %s in namespace %s", service.Name, service.Namespace)
		return reconcile.Result{}, err
	}

	r.updateServiceStatus(serviceIpAddress, service)
	return reconcile.Result{}, nil
}

func (r *ReconcileService) updateServiceStatus(serviceIpAddress string, service *corev1.Service) {

}
