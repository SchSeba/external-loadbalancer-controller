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

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)


func LoadNodes(kubeClient *kubernetes.Clientset) ([]string,map[string]string) {
	nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	nodeList := make([]string, 0)
	nodeMap := make(map[string]string)
	for _, nodeInstance := range nodes.Items {
		for _, IpAddr := range nodeInstance.Status.Addresses {
			if IpAddr.Type == "InternalIP" {
				if value, ok := nodeMap[nodeInstance.Name]; !ok || value != IpAddr.Address {
					nodeMap[nodeInstance.Name] = IpAddr.Address
					nodeList = append(nodeList, IpAddr.Address)
				}
			}
		}
	}

	return nodeList, nodeMap
}

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, kubeClient *kubernetes.Clientset) error {

	nodeList, nodeMap := LoadNodes(kubeClient)

	providerController, err := provider.NewProviderController(m, kubeClient, nodeList)
	if err != nil {
		return err
	}

	farmController, err := farm.NewFarmController(m, providerController, kubeClient)
	if err != nil {
		return err
	}

	_, err = node.NewNodeController(m, kubeClient, farmController, nodeMap)
	if err != nil {
		return err
	}

	_, err = service.NewServiceController(m, kubeClient, farmController)
	if err != nil {
		return err
	}

	return nil
}
