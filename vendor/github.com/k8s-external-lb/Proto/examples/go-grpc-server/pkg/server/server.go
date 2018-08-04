package server

import (
	"fmt"
	"github.com/cloudflare/cfssl/log"
	pb "github.com/k8s-external-lb/Proto"
	"net"
	"google.golang.org/grpc"
	"context"
)

var GrpcServer = grpc.NewServer()

type ExternalLoadbalancerGrpcServer struct {
	ipAddrCount int
	farmList map[string]string
}


func (g *ExternalLoadbalancerGrpcServer)Create(ctx context.Context,data *pb.Data) (*pb.Result, error) {
	log.Debug("Create function")
	log.Debugf("%+v\n",data)

	result := &pb.Result{FarmAddress:fmt.Sprintf("10.0.0.%d",g.ipAddrCount)}
	g.ipAddrCount ++

	g.farmList[data.FarmName] = result.FarmAddress
	return result, nil
}

func (g *ExternalLoadbalancerGrpcServer)Update(ctx context.Context,data *pb.Data) (*pb.Result, error) {
	log.Debug("Update function")
	log.Debugf("%+v\n",data)

	if  value, ok := g.farmList[data.FarmName]; ok {
		return &pb.Result{FarmAddress:value}, nil
	}

	return nil,fmt.Errorf("Fail to update farm, the farm doesnt exist")
}

func (g *ExternalLoadbalancerGrpcServer)Delete(ctx context.Context,data *pb.Data) (*pb.Result, error) {
	log.Debug("Delete function")
	log.Debugf("%+v\n",data)

	if  value, ok := g.farmList[data.FarmName]; ok {
		result := &pb.Result{FarmAddress:value}
		delete(g.farmList, data.FarmName)
		return result, nil
	}

	return nil,fmt.Errorf("Fail to delete farm, the farm doesnt exist")
}

func (g *ExternalLoadbalancerGrpcServer)NodesChange(ctx context.Context,nodes *pb.Nodes) (*pb.Result, error) {
	log.Debug("NodeChange function")
	log.Debugf("%+v\n",nodes)

	return &pb.Result{},nil
}

func StartServer() {
	server := &ExternalLoadbalancerGrpcServer{farmList:make(map[string]string),ipAddrCount:1}
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 8050))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	fmt.Printf("Start lisening on 0.0.0.0:%d", 8050)
	pb.RegisterExternalLoadBalancerServer(GrpcServer,server)
	GrpcServer.Serve(lis)
}

func StopServer() {
	GrpcServer.Stop()
}
