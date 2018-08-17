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

package node

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
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/service"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

)

type NodeController struct {
	Controller    controller.Controller
	ReconcileNode reconcile.Reconciler
}

func NewNodeController(mgr manager.Manager, kubeClient *kubernetes.Clientset, serviceController *service.ServiceController) (*NodeController, error) {
	reconcileNode := newReconciler(mgr, kubeClient, serviceController)

	controllerInstance, err := newNodeControllerController(mgr, reconcileNode)
	if err != nil {
		return nil, err
	}
	nodeController := &NodeController{Controller: controllerInstance,
		ReconcileNode: reconcileNode}

	return nodeController, nil

}


func loadNodes(kubeClient *kubernetes.Clientset) (map[string]string) {
	nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	nodeMap := make(map[string]string)
	for _, nodeInstance := range nodes.Items {
		for _, IpAddr := range nodeInstance.Status.Addresses {
			if IpAddr.Type == "InternalIP" {
				if value, ok := nodeMap[nodeInstance.Name]; !ok || value != IpAddr.Address {
					nodeMap[nodeInstance.Name] = IpAddr.Address
				}
			}
		}
	}

	return nodeMap
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kubeClient *kubernetes.Clientset, serviceController *service.ServiceController) *ReconcileNode {


	return &ReconcileNode{Client: mgr.GetClient(),
		kubeClient:     kubeClient,
		serviceController: serviceController,
		scheme:         mgr.GetScheme(),
		Event:          mgr.GetRecorder(managerv1alpha1.EventRecorderName),
		NodeMap:        loadNodes(kubeClient)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newNodeControllerController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("node-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to Node
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

var _ reconcile.Reconciler = &ReconcileNode{}

// ReconcileNode reconciles a Node object
type ReconcileNode struct {
	client.Client
	kubeClient     *kubernetes.Clientset
	Event          record.EventRecorder
	serviceController *service.ServiceController
	scheme         *runtime.Scheme
	NodeMap        map[string]string
}

// Reconcile reads that state of the cluster for a Node object and makes changes based on the state read
// and what is in the Node.Spec
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
func (r *ReconcileNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Node instance
	instance := &corev1.Node{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO: need to implement this
			log.Log.Info("Remove node")
			return reconcile.Result{}, nil
		}

		log.Log.Errorf("Fail to reconcile node error message: %s", err.Error())

		return reconcile.Result{}, err
	}

	// TODO:(seba) Update services on node change
	return reconcile.Result{}, nil
}
