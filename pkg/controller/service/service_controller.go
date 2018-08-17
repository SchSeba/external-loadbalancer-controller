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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"

	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/farm"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"time"
)

type ServiceController struct {
	Controller       controller.Controller
	ReconcileService *ReconcileService
}

func (s *ServiceController)UpdateAllServices() {
	services, err := s.ReconcileService.kubeClient.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		log.Log.Errorf("Fail to get all services error: %v",err)
	}

	for _,service := range services.Items {
		s.ReconcileService.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: service.Namespace, Name: service.Name}})
	}
}


func NewServiceController(mgr manager.Manager, kubeClient *kubernetes.Clientset, farmController *farm.FarmController) (*ServiceController, error) {
	reconcileService := newReconciler(mgr, kubeClient, farmController)

	controllerInstance, err := newController(mgr, reconcileService)
	if err != nil {
		return nil, err
	}
	serviceController := &ServiceController{Controller: controllerInstance,
		ReconcileService: reconcileService}

	go reconcileService.reSyncProcess()

	return serviceController, nil

}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kubeClient *kubernetes.Clientset, farmController *farm.FarmController) *ReconcileService {
	return &ReconcileService{Client: mgr.GetClient(),
		kubeClient:     kubeClient,
		scheme:         mgr.GetScheme(),
		Event:          mgr.GetRecorder(managerv1alpha1.EventRecorderName),
		FarmController: farmController}
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
	kubeClient     *kubernetes.Clientset
	Event          record.EventRecorder
	FarmController *farm.FarmController
	scheme         *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// +kubebuilder:rbac:groups=core,resources=services,verbs=create;get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;update;delete;patch
func (r *ReconcileService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Service instance
	service := &corev1.Service{}
	err := r.Get(context.TODO(), request.NamespacedName, service)
	if err != nil {
		if errors.IsNotFound(err) {
			r.FarmController.DeleteFarm(request.Namespace, request.Name)
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if service.Spec.Type != "LoadBalancer" || len(service.Finalizers) != 0 {
		return reconcile.Result{}, nil
	}

	if r.FarmController.CreateOrUpdateFarm(service) {
		_, err := r.kubeClient.CoreV1().Services(service.Namespace).UpdateStatus(service)
		if err != nil {
			log.Log.Errorf("Fail to update service status error message: %s", err.Error())
		} else {
			r.FarmController.UpdateSuccessEventOnService(service, "Successfully create/update service on provider")
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileService) UpdateEndpoints(endpoint *corev1.Endpoints) {
	service, err := r.getServiceFromEndpoint(endpoint)
	if err != nil {
		log.Log.Errorf("fail to find service for endpoint %s in namespace %s error: %v", endpoint.Name, endpoint.Namespace, err)
		return
	}

	r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: service.Namespace, Name: service.Name}})
}

func (r *ReconcileService) getServiceFromEndpoint(endpointInstance *corev1.Endpoints) (*corev1.Service, error) {
	return r.kubeClient.CoreV1().Services(endpointInstance.Namespace).Get(endpointInstance.Name, metav1.GetOptions{})
}

func (r *ReconcileService) reSyncProcess() {
	resyncTick := time.Tick(30 * time.Second)

	labelSelector := labels.Set{}
	labelSelector[managerv1alpha1.ServiceStatusLabel] = managerv1alpha1.ServiceStatusLabelFailed

	for range resyncTick {
		var serviceList corev1.ServiceList
		err := r.Client.List(context.TODO(), &client.ListOptions{LabelSelector: labelSelector.AsSelector()}, &serviceList)
		if err != nil {
			log.Log.Error("reSyncProcess: Fail to get Service list")
		} else {
			for _, service := range serviceList.Items {
				r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: service.Namespace, Name: service.Name}})
			}
		}
	}
}
