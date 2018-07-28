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
	"google.golang.org/grpc"
	"time"
)

const (
	EventRecorderName   = "External-Loadbalancer"
	ControllerNamespace = "external-loadbalancer"

	ExternalLoadbalancerAnnotationKey = "external.loadbalancer/provider"
	ExternalLoadbalancerDefaultLabel  = "external-loadbalancer-default"

	ProviderConnectionStatusFail    = "Failed"
	ProviderConnectionStatusSuccess = "Synced"

	FarmStatusLabel = "external-loadbalancer-farm-status"
	FarmStatusLabelSynced = "Synced"
	FarmStatusLabelFailed = "Failed"
	FarmStatusLabelDeleted= "Deleted"


)

var (
	GrpcDial    = grpc.WithInsecure()
	GrpcTimeout = 30 * time.Second
)
