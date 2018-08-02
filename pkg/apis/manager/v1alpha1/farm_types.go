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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FarmSpec defines the desired state of Farm
type FarmSpec struct {
	ServiceName      string `json:"serviceName"`
	ServiceNamespace string `json:"serviceNamespace"`
	Provider         string `json:"provider"`
	Ports            []corev1.ServicePort `json:"ports"`
}

// FarmStatus defines the observed state of Farm
type FarmStatus struct {
	IpAdress         string      `json:"ipAdress"`
	NodeList         []string    `json:"nodeList"`
	ConnectionStatus string      `json:"connectionStatus"`
	LastUpdate       metav1.Time `json:"lastUpdate"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Farm is the Schema for the farms API
// +k8s:openapi-gen=true
type Farm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FarmSpec   `json:"spec,omitempty"`
	Status FarmStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FarmList contains a list of Farm
type FarmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Farm `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Farm{}, &FarmList{})
}

func (f *Farm)FarmName() (string) {
	return fmt.Sprintf("%s-%s",f.Namespace,f.Name)
}

func CreateFarmObject(service *corev1.Service,farmName,providerName string) *Farm {
	return &Farm{ObjectMeta:metav1.ObjectMeta{Name:farmName},
			Spec:FarmSpec{ServiceName:service.Name,
			ServiceNamespace:service.Namespace,
			Ports:service.Spec.Ports,
			Provider:providerName}}
}