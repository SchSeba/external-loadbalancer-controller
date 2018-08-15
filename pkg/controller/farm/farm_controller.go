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
	"reflect"
	"time"

	managerv1alpha1 "github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/provider"
	"k8s.io/client-go/kubernetes"
)

type FarmController struct {
	Client             client.Client
	Controller         controller.Controller
	providerController *provider.ProviderController
	kubeClient         *kubernetes.Clientset
	ReconcileFarm      *ReconcileFarm
}

func NewFarmController(mgr manager.Manager, providerController *provider.ProviderController, kubeClient *kubernetes.Clientset) (*FarmController, error) {
	reconcileFarm := newReconciler(mgr, providerController)
	controllerInstance, err := newController(mgr, reconcileFarm)
	if err != nil {
		return nil, err
	}

	farmController := &FarmController{Client: mgr.GetClient(), Controller: controllerInstance, providerController: providerController, ReconcileFarm: reconcileFarm, kubeClient: kubeClient}
	go farmController.CleanRemovedServices()
	go farmController.reSyncFailFarms()

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
func newReconciler(mgr manager.Manager, providerController *provider.ProviderController) *ReconcileFarm {
	return &ReconcileFarm{Client: mgr.GetClient(),
		providerController: providerController,
		scheme:             mgr.GetScheme(),
		Event:              mgr.GetRecorder(managerv1alpha1.EventRecorderName)}
}

func (f *FarmController) GetFarm(farmName string) (*managerv1alpha1.Farm, error) {
	farm := &managerv1alpha1.Farm{}
	err := f.Client.Get(context.TODO(), client.ObjectKey{Namespace: managerv1alpha1.ControllerNamespace, Name: farmName}, farm)
	if err != nil {
		return nil, err
	}

	return farm, nil
}

func (f *FarmController) CreateOrUpdateFarm(service *corev1.Service) bool {
	farmName := fmt.Sprintf("%s-%s", service.Namespace, service.Name)
	farm := &managerv1alpha1.Farm{}

	err := f.Client.Get(context.TODO(), client.ObjectKey{Namespace: managerv1alpha1.ControllerNamespace, Name: farmName}, farm)
	if err != nil {
		if errors.IsNotFound(err) {
			f.createFarm(service)
			return true
		}
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to get farm object for service %s on namespace %s", service.Name, service.Namespace))
		return true
	}

	needToUpdate, err := f.needToUpdate(farm, service)
	if err != nil {
		return true
	}

	if needToUpdate {
		f.updateFarm(farm, service)
		return true
	}

	return f.needToAddIngressIpFromFarm(service, farm)
}

func (f *FarmController) needToAddIngressIpFromFarm(service *corev1.Service, farm *managerv1alpha1.Farm) bool {
	ingressList := []corev1.LoadBalancerIngress{}

	for _, externalIP := range service.Spec.ExternalIPs {
		ingressList = append(ingressList, corev1.LoadBalancerIngress{IP: externalIP})
	}

	ingressList = append(ingressList, corev1.LoadBalancerIngress{IP: farm.Status.IpAdress})

	if !reflect.DeepEqual(ingressList, service.Status.LoadBalancer.Ingress) {
		service.Status.LoadBalancer.Ingress = ingressList
		return true
	}
	return false
}

func (f *FarmController) createFarm(service *corev1.Service) {
	providerInstance, err := f.getProvider(service)
	if err != nil {
		log.Log.V(2).Errorf("Fail to find provider for service %s on namespace %s", service.Name, service.Namespace)
		f.markServiceStatusFail(service, "Fail to find a provider for the service")
	}

	farm,err := f.createFarmObject(service, fmt.Sprintf("%s-%s", service.Namespace, service.Name), providerInstance)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to create farm error: %v", err))
		return
	}

	farmIpAddress, err := f.providerController.CreateFarm(farm)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to create farm on provider error: %s", err.Error()))
		return
	}

	errCreateFarm := f.Client.Create(context.Background(), farm)
	if errCreateFarm != nil {
		log.Log.V(2).Errorf("Fail to create farm error message: %s", errCreateFarm.Error())
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to create farm error message: %s", errCreateFarm.Error()))
	}

	if err != nil {
		log.Log.V(2).Errorf("Fail to create farm  on provider %s error message: %s", farm.Spec.Provider, errCreateFarm.Error())
		f.FarmUpdateFailStatus(farm, "Warning", "FarmCreatedFail", err.Error())
	}

	f.FarmUpdateSuccessStatus(farm, farmIpAddress, "Normal", "FarmCreated", fmt.Sprintf("Farm created on provider %s", farm.Spec.Provider))
	err = f.Client.Update(context.Background(), farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to update farm status error message: %s", errCreateFarm.Error())
		return
	}

	f.updateServiceIpAddress(service, farmIpAddress)
}

func (f *FarmController) markServiceStatusFail(service *corev1.Service, message string) {
	f.ReconcileFarm.Event.Event(service.DeepCopyObject(), "Warning", "FarmCreatedFail", message)
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}
	service.Labels[managerv1alpha1.ServiceStatusLabel] = managerv1alpha1.ServiceStatusLabelFailed
}

func (f *FarmController) updateFarm(farm *managerv1alpha1.Farm, service *corev1.Service) {
	providerInstance, err := f.getProvider(service)
	if err != nil {
		log.Log.V(2).Errorf("Fail to find provider for service %s on namespace %s", service.Name, service.Namespace)
		f.markServiceStatusFail(service, "Fail to find a provider for the service")
	}

	if farm.Spec.Provider != providerInstance.Name {
		err = f.providerController.DeleteFarm(farm)
		if err != nil {
			deletedProviderFarm, err := f.createFarmObject(service,
				fmt.Sprintf("%s-%s-%s", service.Namespace,
					providerInstance.Name,
					service.Name), providerInstance)
			if err != nil {
				log.Log.Errorf("fail to create a new farm for a delete service object error %v",err)
			}

			f.FarmUpdateFailDeleteStatus(deletedProviderFarm, "Warning", "FarmDeleteFail", err.Error())
			err = f.Client.Update(context.Background(), deletedProviderFarm)
			if err != nil {
				log.Log.V(2).Error("Fail to create a new farm for for the deleted farm on provider")
			}
		}

		delete(service.Labels, managerv1alpha1.ServiceStatusLabel)
		f.Client.Delete(context.Background(), farm)
		f.createFarm(service)
		return
	}

	farm.Spec.Ports = service.Spec.Ports
	nodelist, err := f.getNodeList(service,providerInstance)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to update farm on provider error: %s", err.Error()))
		return
	}

	farm.Status.NodeList = nodelist
	farmIpAddress, err := f.providerController.UpdateFarm(farm)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to update farm on provider error: %s", err.Error()))
		return
	}

	errCreateFarm := f.Client.Update(context.Background(), farm)
	if errCreateFarm != nil {
		log.Log.V(2).Errorf("Fail to update farm error message: %s", errCreateFarm.Error())
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to update farm error message: %s", errCreateFarm.Error()))
	}

	if err != nil {
		log.Log.V(2).Errorf("Fail to update farm  on provider %s error message: %s", farm.Spec.Provider, errCreateFarm.Error())
		f.FarmUpdateFailStatus(farm, "Warning", "FarmUpdateFail", err.Error())
	}

	f.FarmUpdateSuccessStatus(farm, farmIpAddress, "Normal", "FarmUpdate", fmt.Sprintf("Farm updated on provider %s", farm.Spec.Provider))
	err = f.Client.Update(context.Background(), farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to update farm status error message: %s", errCreateFarm.Error())
		return
	}

	delete(service.Labels, managerv1alpha1.ServiceStatusLabel)
	f.updateServiceIpAddress(service, farmIpAddress)
	log.Log.Infof("Successfully create a farm %s for service %s on provider %s",farm.Name,service.Name,providerInstance.Name)
}

func (f *FarmController) DeleteFarm(serviceNamespace, serviceName string) {
	farm := &managerv1alpha1.Farm{}
	err := f.Client.Get(context.Background(), client.ObjectKey{Name: fmt.Sprintf("%s-%s", serviceNamespace, serviceName), Namespace: managerv1alpha1.ControllerNamespace}, farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to find farm %s-%s for deletion", serviceName, serviceNamespace)
		return
	}

	err = f.providerController.DeleteFarm(farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to delete farm on provider %s error message: %s", farm.Spec.Provider, err.Error())
		f.FarmUpdateFailDeleteStatus(farm, "Warning", "FarmDeleteFail", err.Error())
		err = f.Client.Update(context.Background(), farm)
		if err != nil {
			log.Log.V(2).Errorf("Fail to update delete label on farm %s", farm.Name)
		}

		return
	}

	err = f.Client.Delete(context.Background(), farm)
	if err != nil {
		log.Log.V(2).Errorf("Fail to delete farm %s", farm.Name)
	}
}

func (f *FarmController) updateServiceIpAddress(service *corev1.Service, farmIpAddress string) {
	ingressList := []corev1.LoadBalancerIngress{}

	for _, externalIP := range service.Spec.ExternalIPs {
		ingressList = append(ingressList, corev1.LoadBalancerIngress{IP: externalIP})
	}

	ingressList = append(ingressList, corev1.LoadBalancerIngress{IP: farmIpAddress})
	service.Status.LoadBalancer.Ingress = ingressList
}

func (f *FarmController) updateLabels(farm *managerv1alpha1.Farm, status string) {
	if farm.Labels == nil {
		farm.Labels = make(map[string]string)
	}
	farm.Labels[managerv1alpha1.FarmStatusLabel] = status
	farm.Status.ConnectionStatus = status
	farm.Status.LastUpdate = metav1.NewTime(time.Now())
}

func (f *FarmController) FarmUpdateFailStatus(farm *managerv1alpha1.Farm, eventType, reason, message string) {
	f.ReconcileFarm.Event.Event(farm.DeepCopyObject(), eventType, reason, message)
	f.updateLabels(farm, managerv1alpha1.FarmStatusLabelFailed)
}

func (f *FarmController) FarmUpdateFailDeleteStatus(farm *managerv1alpha1.Farm, eventType, reason, message string) {
	f.ReconcileFarm.Event.Event(farm.DeepCopyObject(), eventType, reason, message)
	f.updateLabels(farm, managerv1alpha1.FarmStatusLabelDeleted)
}

func (f *FarmController) FarmUpdateSuccessStatus(farm *managerv1alpha1.Farm, ipAddress, eventType, reason, message string) {
	f.ReconcileFarm.Event.Event(farm.DeepCopy(), eventType, reason, message)
	f.updateLabels(farm, managerv1alpha1.FarmStatusLabelSynced)
	farm.Status.IpAdress = ipAddress
}

func (f *FarmController) needToUpdate(farm *managerv1alpha1.Farm, service *corev1.Service) (bool, error) {
	providerInstance, err := f.getProvider(service)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to get provider for service %s in namespace %s error: %v",service.Name,service.Namespace, err))
		return false, err
	}

	if farm.Spec.Provider != providerInstance.Name {
		return true, nil
	}

	if _, ok := service.Labels[managerv1alpha1.ServiceStatusLabel]; ok {
		return true, nil
	}

	if !reflect.DeepEqual(farm.Spec.Ports, service.Spec.Ports) {
		return true, nil
	}

	nodeList, err := f.getNodeList(service,providerInstance)
	if err != nil {
		f.markServiceStatusFail(service, fmt.Sprintf("Fail to get node lists for service %s in namespace %s error: %v",service.Name,service.Namespace, err))
		return false, err
	}

	if !reflect.DeepEqual(farm.Status.NodeList, nodeList) {
		return true, nil
	}

	return false, nil
}

func (f *FarmController) getServiceFromFarm(farmInstance *managerv1alpha1.Farm) (*corev1.Service, error) {
	return f.kubeClient.CoreV1().Services(farmInstance.Spec.ServiceNamespace).Get(farmInstance.Spec.ServiceName, metav1.GetOptions{})
}

func (f *FarmController) serviceExist(farmInstance *managerv1alpha1.Farm) bool {
	_, err := f.getServiceFromFarm(farmInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		log.Log.V(2).Errorf("fail to get service %s on namespace %s from farm with error message %s", farmInstance.Spec.ServiceNamespace, farmInstance.Spec.ServiceName, err.Error())
	}

	return true
}

func(f *FarmController) getEndPoints(service *corev1.Service) ([]string,error){
	endpointsList := make([]string,0)
	endpoints, err := f.kubeClient.CoreV1().Endpoints(service.Namespace).Get(service.Name,metav1.GetOptions{})
	if err != nil {
		return endpointsList,err
	}

	for _,endpointValue :=range endpoints.Subsets[0].Addresses {
		endpointsList= append(endpointsList,endpointValue.IP)
	}

	return endpointsList, nil
}

func (f *FarmController) getClusterNodes() ([]string,error) {
	nodeList := make([]string, 0)
	nodes, err := f.kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nodeList, err
	}

	for _, nodeInstance := range nodes.Items {
		for _, IpAddr := range nodeInstance.Status.Addresses {
			if IpAddr.Type == "InternalIP" {
				nodeList = append(nodeList, IpAddr.Address)
			}
		}
	}

	return nodeList,nil
}

func (f *FarmController) reSyncFailFarms() {
	resyncTick := time.Tick(30 * time.Second)

	for range resyncTick {
		var farmList managerv1alpha1.FarmList

		// Sync farm need to be deleted
		labelSelector := labels.Set{}
		labelSelector[managerv1alpha1.FarmStatusLabel] = managerv1alpha1.FarmStatusLabelDeleted
		err := f.Client.List(context.TODO(), &client.ListOptions{LabelSelector: labelSelector.AsSelector()}, &farmList)
		if err != nil {
			log.Log.V(2).Error("reSyncProcess: Fail to get farm list")
		} else {
			for _, farmInstance := range farmList.Items {
				if !f.serviceExist(&farmInstance) {
					f.DeleteFarm(farmInstance.Spec.ServiceNamespace, farmInstance.Spec.ServiceName)
				} else {
					service, err := f.getServiceFromFarm(&farmInstance)
					if err != nil {
						log.Log.V(2).Errorf("fail to get service %s on namespace %s from farm with error message %s", farmInstance.Spec.ServiceNamespace, farmInstance.Spec.ServiceName, err.Error())
					}
					f.updateFarm(&farmInstance, service)
				}
			}
		}
	}
}

func (f *FarmController) CleanRemovedServices() {
	cleanTick := time.NewTimer(10 * time.Minute)

	for range cleanTick.C {
		var farmList = managerv1alpha1.FarmList{}
		err := f.Client.List(context.TODO(), nil, &farmList)
		if err != nil {
			log.Log.V(2).Error("CleanRemovedServices: Fail to get farm list")
		} else {
			service := &corev1.Service{}
			for _, farmInstance := range farmList.Items {
				err := f.Client.Get(context.Background(), client.ObjectKey{Name: farmInstance.Spec.ServiceName, Namespace: farmInstance.Spec.ServiceNamespace}, service)
				if err != nil && errors.IsNotFound(err) {
					f.DeleteFarm(farmInstance.Spec.ServiceNamespace, farmInstance.Spec.ServiceName)
				}
			}
		}
	}
}

func (f *FarmController) getProvider(service *corev1.Service) (*managerv1alpha1.Provider, error) {
	var providerInstance managerv1alpha1.Provider

	var err error

	if value, ok := service.ObjectMeta.Annotations[managerv1alpha1.ExternalLoadbalancerAnnotationKey]; ok {
		err = f.Client.Get(context.TODO(), client.ObjectKey{Name: value, Namespace: managerv1alpha1.ControllerNamespace}, &providerInstance)
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

		providerInstance = providerList.Items[0]
	}

	return &providerInstance, nil
}

func (f *FarmController) createFarmObject(service *corev1.Service, farmName string, provider *managerv1alpha1.Provider) (*managerv1alpha1.Farm, error) {
	farmStatus, err := f.farmDefaultStatus(service,provider)
	if err != nil {
		return nil , err
	}
	return &managerv1alpha1.Farm{ObjectMeta: metav1.ObjectMeta{Name: farmName, Namespace: managerv1alpha1.ControllerNamespace},
		Spec: managerv1alpha1.FarmSpec{ServiceName: service.Name,
			ServiceNamespace: service.Namespace,
			Ports:            service.Spec.Ports,
			Provider:         provider.Name}, Status: *farmStatus },nil
}

func(f *FarmController) farmDefaultStatus(service *corev1.Service,provider *managerv1alpha1.Provider) (*managerv1alpha1.FarmStatus, error) {
	nodelist, err := f.getNodeList(service,provider)
	if err != nil {
		return nil, err
	}

	return &managerv1alpha1.FarmStatus{NodeList: nodelist, IpAdress: "", LastUpdate: metav1.NewTime(time.Now()), ConnectionStatus: ""}, err
}

func (f *FarmController) getNodeList(service *corev1.Service,provider *managerv1alpha1.Provider) ([]string,error) {
	if provider.Spec.Internal == true {
		return f.getEndPoints(service)
	}

	return f.getClusterNodes()
}

var _ reconcile.Reconciler = &ReconcileFarm{}

// ReconcileFarm reconciles a Farm object
type ReconcileFarm struct {
	client.Client
	providerController *provider.ProviderController
	Event              record.EventRecorder
	scheme             *runtime.Scheme
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

	//log.Log.Infof("%+v", instance)
	return reconcile.Result{}, nil
}