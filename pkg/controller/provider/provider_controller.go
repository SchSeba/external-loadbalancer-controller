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

package provider

import (
	"context"
	"fmt"
	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	. "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/grpc-client"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

type ProviderController struct {
	Controller        controller.Controller
	ReconcileProvider *ReconcileProvider
}

func NewProviderController(mgr manager.Manager, kubeClient *kubernetes.Clientset) (*ProviderController, error) {
	reconcileProvider := newReconciler(mgr, kubeClient)
	controllerInstance, err := newController(mgr, reconcileProvider)
	if err != nil {
		return nil, err
	}

	providerController := &ProviderController{Controller: controllerInstance,
		ReconcileProvider: reconcileProvider}

	return providerController, nil
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, kubeClient *kubernetes.Clientset) *ReconcileProvider {
	return &ReconcileProvider{Client: mgr.GetClient(),
		kubeClient: kubeClient,
		scheme:     mgr.GetScheme(),
		Event:      mgr.GetRecorder(managerv1alpha1.EventRecorderName)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func newController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("provider-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &managerv1alpha1.Provider{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (p *ProviderController) getProvider(farm *managerv1alpha1.Farm) (*managerv1alpha1.Provider, error) {
	provider := managerv1alpha1.Provider{}
	err := p.ReconcileProvider.Client.Get(context.TODO(),
		client.ObjectKey{Name: farm.Spec.Provider,
			Namespace: managerv1alpha1.ControllerNamespace},
		&provider)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func (p *ProviderController) CreateFarm(farm *managerv1alpha1.Farm) (string, error) {
	provider, err := p.getProvider(farm)
	if err != nil {
		return "", err
	}

	farmIpAddress, err := Grpc.CreateFarm(provider.Spec.Url, farm)

	if err != nil {
		log.Log.V(2).Errorf("Fail to create farm: %s on provider %s error message: %s", farm.FarmName(), provider.Name, err.Error())
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmCreateFail", err.Error())
		return "", err
	}

	p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmCreateSuccess", fmt.Sprintf("Farm %s-%s was created on provider", farm.Namespace, farm.Name))
	return farmIpAddress, nil
}

func (p *ProviderController) UpdateFarm(farm *managerv1alpha1.Farm) (string, error) {
	provider, err := p.getProvider(farm)
	if err != nil {
		return "", err
	}

	farmIpAddress, err := Grpc.UpdateFarm(provider.Spec.Url, farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to update farm: %s on provider %s error message: %s", farm.FarmName(), provider.Name, err.Error())
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmUpdateFail", err.Error())
		return "", err
	}

	log.Log.V(2).Infof("successfully updated farm: %s on provider %s", farm.FarmName(), provider.Name)
	p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmCreateSuccess", fmt.Sprintf("Farm %s-%s was updated on provider", farm.Namespace, farm.Name))
	return farmIpAddress, nil
}

func (p *ProviderController) DeleteFarm(farm *managerv1alpha1.Farm) error {
	provider, err := p.getProvider(farm)
	if err != nil {
		return err
	}
	err = Grpc.RemoveFarm(provider.Spec.Url, farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to remove farm: %s on provider %s error message: %s", farm.FarmName(), provider.Name, err.Error())
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmDeleteFail", err.Error())
		return err
	}

	log.Log.V(2).Infof("successfully removed farm: %s on provider %s", farm.FarmName(), provider.Name)
	p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmDeleteSuccess", fmt.Sprintf("Farm %s-%s was deleted on provider", farm.Namespace, farm.Name))
	return nil
}

//func (p *ProviderController) oldUpdateFarm(farm *managerv1alpha1.Farm) (string, error) {
//	provider, err := p.getProvider(farm)
//	if err != nil {
//		return "", nil
//	}
//
//	farmIpAddress, err := grpcClient.UpdateFarm(provider.Spec.Url, farm)
//	if err != nil {
//		p.ProviderUpdateFailStatus(provider, "Warning", "FarmUpdated", err.Error())
//		p.FarmUpdateFailStatus(farm, "Warning", "FarmCreated", err.Error())
//		return "", err
//	}
//	p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmUpdated", fmt.Sprintf("Farm %s-%s updated on provider", farm.Namespace, farm.Name))
//	p.FarmUpdateSuccessStatus(farm, farmIpAddress, "Normal", "FarmUpdated", fmt.Sprintf("Farm updated on provider %s", provider.Name))
//
//	farm.Status.NodeList = p.ReconcileProvider.NodeList
//	p.ReconcileProvider.Client.Update(context.Background(), farm)
//
//	return farmIpAddress, nil
//}

//func (p *ProviderController) oldDeleteFarm(farmName string) {
//	farmInstance, err := p.ReconcileProvider.farmController.GetFarm(farmName)
//	if err != nil {
//		log.Error("Fail to get farm error: ", err)
//		return
//	}
//
//	err = p.removeFarm(farmInstance)
//	if err != nil {
//		log.Error("Fail to delete farm error: ", err)
//		return
//	}
//}
//
//func (p *ProviderController) removeFarm(farm *managerv1alpha1.Farm) error {
//	provider, err := p.getProvider(farm)
//	if err != nil {
//		return nil
//	}
//
//	err = grpcClient.RemoveFarm(provider.Spec.Url, farm)
//	if err != nil {
//		p.FarmUpdateFailDeleteStatus(farm, "Warning", "FarmDeleted", err.Error())
//		p.ProviderUpdateFailStatus(provider, "Warning", "FarmDeleted", fmt.Sprint("Fail to delete farm error: ", err))
//		return err
//	}
//
//	err = p.ReconcileProvider.Client.Delete(context.TODO(), farm)
//	if err != nil {
//		p.ProviderUpdateFailStatus(provider, "Warning", "FarmDeleted", fmt.Sprint("Fail to delete farm error: ", err))
//		return err
//	}
//
//	return nil
//}

//
//func (p *ProviderController)updateProviderStatus(err error) {
//
//}
//

func (p *ProviderController) updateLabels(provider *managerv1alpha1.Provider, status string) {
	if provider.Labels == nil {
		provider.Labels = make(map[string]string)
	}
	provider.Status.ConnectionStatus = status
	provider.Status.LastUpdate = metav1.NewTime(time.Now())
	p.ReconcileProvider.Client.Update(context.TODO(), provider)
}

func (p *ProviderController) ProviderUpdateFailStatus(provider *managerv1alpha1.Provider, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(provider.DeepCopyObject(), eventType, reason, message)
	p.updateLabels(provider, managerv1alpha1.ProviderConnectionStatusFail)
}

func (p *ProviderController) ProviderUpdateSuccessStatus(provider *managerv1alpha1.Provider, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(provider.DeepCopy(), eventType, reason, message)
	p.updateLabels(provider, managerv1alpha1.ProviderConnectionStatusSuccess)
}

var _ reconcile.Reconciler = &ReconcileProvider{}

// ReconcileProvider reconciles a Provider object
type ReconcileProvider struct {
	client.Client
	kubeClient *kubernetes.Clientset
	Event      record.EventRecorder
	scheme     *runtime.Scheme
}

// TODO: Change this Shit

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
// +kubebuilder:rbac:groups=manager.external-loadbalancer,resources=providers,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileProvider) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Provider instance
	instance := &managerv1alpha1.Provider{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	//fmt.Printf("%+v\n", instance)
	return reconcile.Result{}, nil
}
