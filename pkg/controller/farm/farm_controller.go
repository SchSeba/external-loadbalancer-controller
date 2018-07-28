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

package farm

import (
	"context"
	"fmt"

	"github.com/cloudflare/cfssl/log"

	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

type FarmController struct {
	Client        client.Client
	Controller    controller.Controller
	ReconcileFarm reconcile.Reconciler
}

func NewFarmController(mgr manager.Manager) (*FarmController, error) {
	reconcileFarm := newReconciler(mgr)
	controllerInstance, err := newController(mgr, reconcileFarm)
	if err != nil {
		return nil, err
	}

	farmController := &FarmController{Client: mgr.GetClient(), Controller: controllerInstance, ReconcileFarm: reconcileFarm}
	return farmController, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("farm-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to Node
	err = c.Watch(&source.Kind{Type: &managerv1alpha1.Farm{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileFarm {
	return &ReconcileFarm{Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		Event:  mgr.GetRecorder(managerv1alpha1.EventRecorderName)}
}

func (f *FarmController) GetOrCreateFarm(service *corev1.Service) (*managerv1alpha1.Farm, bool, error) {
	farmName := fmt.Sprintf("%s-%s", service.Namespace, service.Name)
	farm := &managerv1alpha1.Farm{}
	isCreated := false

	err := f.Client.Get(context.TODO(), client.ObjectKey{Namespace: managerv1alpha1.ControllerNamespace, Name: farmName}, farm)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, false, err
		}
		farm, err = f.createFarm(farmName, service)
		if err != nil {
			return nil, false, err
		}

		isCreated = true
	}

	return farm, isCreated, nil
}

func (f *FarmController) GetFarm(farmName string) (*managerv1alpha1.Farm, error) {
	farm := &managerv1alpha1.Farm{}
	err := f.Client.Get(context.TODO(), client.ObjectKey{Namespace: managerv1alpha1.ControllerNamespace, Name: farmName}, farm)
	if err != nil {
		return nil, err
	}

	return farm, nil
}

func (f *FarmController) NeedToUpdate(farm *managerv1alpha1.Farm, service *corev1.Service) bool {
	if reflect.DeepEqual(farm.Spec.Ports, service.Spec.Ports) && service.Status.LoadBalancer.Ingress != nil && service.Status.LoadBalancer.Ingress[0].IP == farm.Status.IpAdress {
		return false
	}

	farm.Spec.Ports = service.Spec.Ports
	farm.Status.ServiceVersion = service.ResourceVersion

	return true
}

func (f *FarmController) getProvider(service *corev1.Service) (*managerv1alpha1.Provider, error) {
	var provider managerv1alpha1.Provider

	var err error

	if value, ok := service.ObjectMeta.Annotations[managerv1alpha1.ExternalLoadbalancerAnnotationKey]; ok {
		err = f.Client.Get(context.TODO(), client.ObjectKey{Name: value, Namespace: managerv1alpha1.ControllerNamespace}, &provider)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Provider Not found for service : %s", service.Name)
			}
			return nil, err
		}

	} else {
		var providerList managerv1alpha1.ProviderList
		labelSelector := labels.Set{}
		labelSelector[managerv1alpha1.ExternalLoadbalancerDefaultLabel] = "true"
		err = f.Client.List(context.TODO(), &client.ListOptions{LabelSelector: labelSelector.AsSelector()}, &providerList)
		if err != nil {
			return nil, err
		}

		if len(providerList.Items) == 0 {
			return nil, fmt.Errorf("Default provider not found")
		} else if len(providerList.Items) > 1 {
			return nil, fmt.Errorf("More then one default provider found")
		}

		provider = providerList.Items[0]
	}

	return &provider, nil
}

func (f *FarmController) createFarm(farmName string, service *corev1.Service) (*managerv1alpha1.Farm, error) {
	provider, err := f.getProvider(service)
	if err != nil {
		return nil, err
	}

	farm := managerv1alpha1.Farm{ObjectMeta: metav1.ObjectMeta{Name: farmName,
		Namespace: managerv1alpha1.ControllerNamespace},
		Spec: managerv1alpha1.FarmSpec{Ports: service.Spec.Ports,
			Provider: provider.Name, ServiceName: service.Name, ServiceNamespace: service.Namespace},
		Status: managerv1alpha1.FarmStatus{ServiceVersion: service.ResourceVersion, NodeList: []string{}, LastUpdate: metav1.NewTime(time.Now())}}

	err = f.Client.Create(context.Background(), &farm)
	if err != nil {
		return nil, err
	}

	return &farm, nil
}

var _ reconcile.Reconciler = &ReconcileFarm{}

// ReconcileFarm reconciles a Farm object
type ReconcileFarm struct {
	Client client.Client
	Event  record.EventRecorder
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Farm object and makes changes based on the state read
// and what is in the Farm.Spec
// +kubebuilder:rbac:groups=manager.external-loadbalancer,resources=farms,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileFarm) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Farm instance
	instance := &managerv1alpha1.Farm{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Infof("%+v", instance)
	return reconcile.Result{}, nil
}
