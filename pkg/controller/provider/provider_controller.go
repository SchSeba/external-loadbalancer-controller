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
	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"fmt"
	grpcClient "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/grpc-client"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

type ProviderController struct {
	Controller        controller.Controller
	ReconcileProvider *ReconcileProvider
}

func NewProviderController(mgr manager.Manager) (*ProviderController, error) {
	reconcileProvider := newReconciler(mgr)
	controllerInstance, err := newController(mgr, reconcileProvider)
	if err != nil {
		return nil, err
	}

	return &ProviderController{Controller: controllerInstance,
		ReconcileProvider: reconcileProvider}, nil
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileProvider {
	return &ReconcileProvider{Client: mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		Event:    mgr.GetRecorder(managerv1alpha1.EventRecorderName),
		NodeList: make([]string, 0)}
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

func (p *ProviderController) getProvider(farm managerv1alpha1.Farm) (*managerv1alpha1.Provider, error) {
	var provider managerv1alpha1.Provider
	err := p.ReconcileProvider.Client.Get(context.TODO(), client.ObjectKey{Name: farm.Name, Namespace: farm.Namespace}, &provider)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Provider Not found for farm : %s", farm.Name)
		}
		return nil, err
	}

	return &provider, nil
}

func (p *ProviderController) CreateFarm(farm managerv1alpha1.Farm) (string, error) {
	provider, err := p.getProvider(farm)
	if err != nil {
		return "", nil
	}

	farmIpAddress, err := grpcClient.CreateFarm(provider.Spec.Url, farm, p.ReconcileProvider.NodeList)
	if err != nil {
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmCreated", err.Error())
		p.FarmUpdateFailStatus(farm, "Warning", "FarmCreated", err.Error())
		return "", err
	} else {
		p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmCreated", fmt.Sprintf("Farm %s-%s created on provider", farm.Namespace, farm.Name))
		p.FarmUpdateSuccessStatus(farm, farmIpAddress, "Normal", "FarmCreated", fmt.Sprintf("Farm created on provider %s", provider.Name))
	}

	return farmIpAddress, nil
}

func (p *ProviderController) UpdateFarm(farm managerv1alpha1.Farm) (string, error) {
	provider, err := p.getProvider(farm)
	if err != nil {
		return "", nil
	}

	farmIpAddress, err := grpcClient.UpdateFarm(provider.Spec.Url, farm)
	if err != nil {
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmUpdated", err.Error())
		p.FarmUpdateFailStatus(farm, "Warning", "FarmCreated", err.Error())
		return "", err
	} else {
		p.ProviderUpdateSuccessStatus(provider, "Normal", "FarmUpdated", fmt.Sprintf("Farm %s-%s updated on provider", farm.Namespace, farm.Name))
		p.FarmUpdateSuccessStatus(farm, farmIpAddress, "Normal", "FarmUpdated", fmt.Sprintf("Farm updated on provider %s", provider.Name))
	}

	return farmIpAddress, nil
}

func (p *ProviderController) RemoveFarm(farm managerv1alpha1.Farm) error {
	provider, err := p.getProvider(farm)
	if err != nil {
		return nil
	}

	err = grpcClient.RemoveFarm(provider.Spec.Url, farm)
	if err != nil {
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmDeleted", err.Error())
		return err
	}

	err = p.ReconcileProvider.Client.Delete(context.TODO(), provider)
	if err != nil {
		p.ProviderUpdateFailStatus(provider, "Warning", "FarmDeleted", err.Error())
		return err
	}

	return nil
}

func (p *ProviderController) UpdateNodes(nodes []string) {
	p.ReconcileProvider.NodeList = nodes

	providers := &managerv1alpha1.ProviderList{}
	p.ReconcileProvider.Client.List(context.TODO(), &client.ListOptions{Namespace: managerv1alpha1.ControllerNamespace}, providers)

	for _, provider := range providers.Items {
		err := grpcClient.UpdateNodes(provider.Spec.Url, p.ReconcileProvider.NodeList)
		if err != nil {
			p.ProviderUpdateFailStatus(&provider, "Warning", "NodeUpdate", err.Error())
		} else {
			p.ProviderUpdateSuccessStatus(&provider, "Normal", "NodeUpdate", "Node list updated successfully")
		}
	}
}

func (p *ProviderController) ProviderUpdateFailStatus(provider *managerv1alpha1.Provider, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(provider.DeepCopyObject(), eventType, reason, message)
	provider.Status.ConnectionStatus = managerv1alpha1.ProviderConnectionStatusFail
	provider.Status.LastUpdate = metav1.Time{}
	p.ReconcileProvider.Client.Update(context.TODO(), provider)

}

func (p *ProviderController) ProviderUpdateSuccessStatus(provider *managerv1alpha1.Provider, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(provider.DeepCopy(), eventType, reason, message)
	provider.Status.ConnectionStatus = managerv1alpha1.ProviderConnectionStatusSuccess
	provider.Status.LastUpdate = metav1.Time{}
	p.ReconcileProvider.Client.Update(context.TODO(), provider)
}

func (p *ProviderController) FarmUpdateFailStatus(farm managerv1alpha1.Farm, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(farm.DeepCopyObject(), eventType, reason, message)
	farm.Status.ConnectionStatus = managerv1alpha1.ProviderConnectionStatusFail
	farm.Status.LastUpdate = metav1.Time{}
	p.ReconcileProvider.Client.Update(context.TODO(), &farm)

}

func (p *ProviderController) FarmUpdateSuccessStatus(farm managerv1alpha1.Farm, ipAddress, eventType, reason, message string) {
	p.ReconcileProvider.Event.Event(farm.DeepCopy(), eventType, reason, message)
	farm.Status.ConnectionStatus = managerv1alpha1.ProviderConnectionStatusSuccess
	farm.Status.LastUpdate = metav1.Time{}
	farm.Status.IpAdress = ipAddress
	farm.Status.NodeList = p.ReconcileProvider.NodeList
	p.ReconcileProvider.Client.Update(context.TODO(), &farm)
}

var _ reconcile.Reconciler = &ReconcileProvider{}

// ReconcileProvider reconciles a Provider object
type ReconcileProvider struct {
	client.Client
	Event    record.EventRecorder
	scheme   *runtime.Scheme
	NodeList []string
}

// TODO: Change this Shit

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
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

	fmt.Printf("%+v\n",instance)
	return reconcile.Result{}, nil

	//// TODO(user): Change this to be the object type created by your controller
	//// Define the desired Deployment object
	//deploy := &appsv1.Deployment{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      instance.Name + "-deployment",
	//		Namespace: instance.Namespace,
	//	},
	//	Spec: appsv1.DeploymentSpec{
	//		Selector: &metav1.LabelSelector{
	//			MatchLabels: map[string]string{"deployment": instance.Name + "-deployment"},
	//		},
	//		Template: corev1.PodTemplateSpec{
	//			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"deployment": instance.Name + "-deployment"}},
	//			Spec: corev1.PodSpec{
	//				Containers: []corev1.Container{
	//					{
	//						Name:  "nginx",
	//						Image: "nginx",
	//					},
	//				},
	//			},
	//		},
	//	},
	//}
	//if err := controllerutil.SetControllerReference(instance, deploy, r.scheme); err != nil {
	//	return reconcile.Result{}, err
	//}
	//
	//// TODO(user): Change this for the object type created by your controller
	//// Check if the Deployment already exists
	//found := &appsv1.Deployment{}
	//err = r.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	//if err != nil && errors.IsNotFound(err) {
	//	log.Debugf("Creating Deployment %s/%s\n", deploy.Namespace, deploy.Name)
	//	err = r.Create(context.TODO(), deploy)
	//	if err != nil {
	//		return reconcile.Result{}, err
	//	}
	//} else if err != nil {
	//	return reconcile.Result{}, err
	//}
	//
	//// TODO(user): Change this for the object type created by your controller
	//// Update the found object and write the result back if there are any changes
	//if !reflect.DeepEqual(deploy.Spec, found.Spec) {
	//	found.Spec = deploy.Spec
	//	log.Debugf("Updating Deployment %s/%s\n", deploy.Namespace, deploy.Name)
	//	err = r.Update(context.TODO(), found)
	//	if err != nil {
	//		return reconcile.Result{}, err
	//	}
	//}
	//return reconcile.Result{}, nil
}
