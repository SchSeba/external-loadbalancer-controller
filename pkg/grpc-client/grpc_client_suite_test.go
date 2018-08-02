package grpc_client_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/golang/mock/gomock"
)

var (
	ctrl *gomock.Controller
)


func TestGrpcClient(t *testing.T) {
	ctrl = gomock.NewController(t)
	defer ctrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "GrpcClient Suite")
}
