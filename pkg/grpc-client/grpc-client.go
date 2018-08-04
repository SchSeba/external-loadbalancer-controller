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

package grpc_client

//go:generate mockgen -source $GOFILE -package=$GOPACKAGE -destination=generated_mock_$GOFILE

/*
 ATTENTION: Rerun code generators when interface signatures are modified.
*/

import (
	pb "github.com/k8s-external-lb/Proto"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"

	"google.golang.org/grpc"

	"context"
)

type GrpcClientInterface interface {
	CreateFarm(string, *v1alpha1.Farm) (string, error)
	UpdateFarm(url string, farm *v1alpha1.Farm) (string, error)
	RemoveFarm(url string, farm *v1alpha1.Farm) error
}

type GrpcClient struct {}

var Grpc = GrpcClient{}

func(g *GrpcClient) getGrpcClient(url string) (pb.ExternalLoadBalancerClient, error) {
	conn, err := grpc.Dial(url, v1alpha1.GrpcDial)
	if err != nil {
		log.Log.Error(err.Error())
		return nil, err
	}

	return pb.NewExternalLoadBalancerClient(conn), nil
}

func (g *GrpcClient) CreateFarm(url string, farm *v1alpha1.Farm) (string, error) {
	client, err := g.getGrpcClient(url)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), v1alpha1.GrpcTimeout)
	defer cancel()

	result, err := client.Create(ctx, &pb.Data{Nodes: farm.Status.NodeList, Ports: createFarmPorts(farm)})
	if err != nil {
		return "",err
	}
	return result.FarmAddress, err
}

func(g *GrpcClient) UpdateFarm(url string, farm *v1alpha1.Farm) (string, error) {
	client, err := g.getGrpcClient(url)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), v1alpha1.GrpcTimeout)
	defer cancel()

	result, err := client.Update(ctx, &pb.Data{Nodes: []string{}, Ports: createFarmPorts(farm)})
	if err != nil {
		 return "",err
	}
	return result.FarmAddress, err
}

func(g *GrpcClient) RemoveFarm(url string, farm *v1alpha1.Farm) error {
	client, err := g.getGrpcClient(url)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), v1alpha1.GrpcTimeout)
	defer cancel()

	_, err = client.Delete(ctx, &pb.Data{FarmName: farm.Name})
	return nil
}

func (g *GrpcClient) UpdateNodes(url string, nodes []string) error {
	client, err := g.getGrpcClient(url)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), v1alpha1.GrpcTimeout)
	defer cancel()

	_, err = client.NodesChange(ctx, &pb.Nodes{List: nodes})
	return err
}

func createFarmPorts(farm *v1alpha1.Farm) []*pb.Port {
	ports := make([]*pb.Port, len(farm.Spec.Ports))
	for idx, port := range farm.Spec.Ports {
		ports[idx] = &pb.Port{Name: port.Name, Protocol: string(port.Protocol), Port: port.Port, NodePort: port.NodePort}
	}

	return ports
}
