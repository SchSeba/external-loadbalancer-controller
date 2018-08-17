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

package endpoint

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
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/service"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

type EndPointController struct {
	Controller    controller.Controller
	ReconcileNode reconcile.Reconciler
}

func NewEndPointController(mgr manager.Manager, kubeClient *kubernetes.Clientset, serviceController *service.ServiceController) (*EndPointController, error) {
	reconcileEndPoint := newReconciler(mgr, kubeClient, serviceController)

	controllerInstance, err := newEndPointController(mgr, reconcileEndPoint)
	if err != nil {
		return nil, err
	}
	endpointController := &EndPointController{Controller: controllerInstance,
		ReconcileNode: reconcileEndPoint}

	return endpointController, nil

}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kubeClient *kubernetes.Clientset, serviceController *service.ServiceController) *ReconcileEndPoint {
	return &ReconcileEndPoint{Client: mgr.GetClient(),
		kubeClient:     kubeClient,
		serviceController: serviceController,
		scheme:         mgr.GetScheme(),
		Event:          mgr.GetRecorder(managerv1alpha1.EventRecorderName)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newEndPointController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("endpoint-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to Node
	err = c.Watch(&source.Kind{Type: &corev1.Endpoints{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

var _ reconcile.Reconciler = &ReconcileEndPoint{}

// ReconcileNode reconciles a Endpoints object
type ReconcileEndPoint struct {
	client.Client
	kubeClient     *kubernetes.Clientset
	Event          record.EventRecorder
	serviceController *service.ServiceController
	scheme         *runtime.Scheme
}

//func (r *ReconcileNode) updateProviderNodeList() error {
//	needToUpdate := false
//	nodeList := make([]string, 0)
//
//	nodes := &corev1.NodeList{}
//	err := r.Client.List(context.Background(), nil, nodes)
//	if err != nil {
//		return err
//	}
//
//	for _, node := range nodes.Items {
//		for _, IpAddr := range node.Status.Addresses {
//			if IpAddr.Type == "InternalIP" {
//				if value, ok := r.NodeMap[node.Name]; !ok || value != IpAddr.Address {
//					needToUpdate = true
//					r.NodeMap[node.Name] = IpAddr.Address
//					nodeList = append(nodeList, IpAddr.Address)
//				}
//			}
//		}
//	}
//
//	err = nil
//	if needToUpdate {
//		r.farmController.CreateNodeList(nodeList)
//	}
//
//	return err
//}

// Reconcile reads that state of the cluster for a Endpoints object and makes changes based on the state read
// and what is in the Endpoints.Spec
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch
func (r *ReconcileEndPoint) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Node instance
	endpoint := &corev1.Endpoints{}
	err := r.Get(context.TODO(), request.NamespacedName, endpoint)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			return reconcile.Result{}, nil
		}

		log.Log.Errorf("Fail to reconcile endpoint error message: %s", err.Error())
		return reconcile.Result{}, err
	}

	if len(endpoint.Subsets) > 0 {
		r.serviceController.ReconcileService.UpdateEndpoints(endpoint)
	}
	return reconcile.Result{}, nil
}
