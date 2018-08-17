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

package controller

import (
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/farm"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/node"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/provider"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/service"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/controller/endpoint"

	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)




// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, kubeClient *kubernetes.Clientset) error {

	providerController, err := provider.NewProviderController(m, kubeClient)
	if err != nil {
		return err
	}

	farmController, err := farm.NewFarmController(m, providerController, kubeClient)
	if err != nil {
		return err
	}

	serviceController, err := service.NewServiceController(m, kubeClient, farmController)
	if err != nil {
		return err
	}

	_, err = node.NewNodeController(m, kubeClient, serviceController)
	if err != nil {
		return err
	}

	_, err = endpoint.NewEndPointController(m, kubeClient, serviceController)
	if err != nil {
		return err
	}
	return nil
}
