package tests_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/k8s-external-lb/external-loadbalancer-controller/tests"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "External LoadBalancer Controller Tests Suite")
}

var _ = BeforeSuite(func() {
	tests.BeforeTestSuitSetup()
})

var _ = AfterSuite(func() {
	tests.AfterTestSuitCleanup()
})