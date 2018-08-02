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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/farm"
)

type NodeController struct {
	Controller    controller.Controller
	ReconcileNode reconcile.Reconciler
}

func NewNodeController(mgr manager.Manager, kubeClient *kubernetes.Clientset, farmController *farm.FarmController,nodeMap map[string]string) (*NodeController, error) {
	reconcileNode := newReconciler(mgr, kubeClient, farmController,nodeMap)

	controllerInstance, err := newController(mgr, reconcileNode)
	if err != nil {
		return nil, err
	}
	nodeController := &NodeController{Controller: controllerInstance,
		ReconcileNode: reconcileNode}

	return nodeController, nil

}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kubeClient *kubernetes.Clientset,farmController *farm.FarmController,nodeMap map[string]string) *ReconcileNode {
	return &ReconcileNode{Client: mgr.GetClient(),
		kubeClient:         kubeClient,
		farmController: farmController,
		scheme:             mgr.GetScheme(),
		Event:              mgr.GetRecorder(managerv1alpha1.EventRecorderName),
		NodeMap:            nodeMap}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
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
	kubeClient         *kubernetes.Clientset
	Event              record.EventRecorder
	farmController *farm.FarmController
	scheme             *runtime.Scheme
	NodeMap            map[string]string
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
			return reconcile.Result{},nil
		}

		log.Log.Errorf("Fail to reconcile node error message: %s", err.Error())

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
