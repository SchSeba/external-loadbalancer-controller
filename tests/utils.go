package tests

import (
	. "github.com/onsi/ginkgo"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/log"
	. "github.com/k8s-external-lb/Proto/examples/go-grpc-server/pkg/server"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/manager"
)

const (
	// tests.NamespaceTestDefault is the default namespace, to test non-infrastructure objects.
	NamespaceTestDefault = "externallb-test-default"
	// NamespaceTestAlternative is used to test controller-namespace independency.
	NamespaceTestAlternative = "externallb-test-alternative"
)


var testNamespaces = []string{NamespaceTestDefault, NamespaceTestAlternative}


func BeforeTestSuitSetup() {

	log.InitializeLogging("tests")
	log.Log.SetIOWriter(GinkgoWriter)

	createNamespaces()
	go manager.StartManager()
}

func AfterTestSuitCleanup() {
	removeNamespace()
}

func GetKubeClient () (*kubernetes.Clientset) {

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	PanicOnError(err)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	PanicOnError(err)

	return kubeClient
}

func GetKubeRestClient() *rest.RESTClient {
	cfg, err := config.GetConfig()
	PanicOnError(err)
	cfg.GroupVersion = &schema.GroupVersion{Group:"manager.external-loadbalancer",Version:"v1alpha1"}
	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	cfg.APIPath = "/apis"
	cfg.ContentType = runtime.ContentTypeJSON
	restClient, err := rest.RESTClientFor(cfg)
	PanicOnError(err)
	return restClient
}

func createNamespaces() {
	kubeCli := GetKubeClient()

	// Create a Test Namespaces
	for _, namespace := range testNamespaces {
		ns := &k8sv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err := kubeCli.CoreV1().Namespaces().Create(ns)
		if !errors.IsAlreadyExists(err) {
			PanicOnError(err)
		}
	}
}

func removeNamespace() {
	kubeCli := GetKubeClient()
	for _, namespace := range testNamespaces {
		err := kubeCli.CoreV1().Namespaces().Delete(namespace,&metav1.DeleteOptions{})
		PanicOnError(err)
	}
}

func StartTestGrpcServer() {
	StartServer()
}

func StopTestGrpcServer() {
	StopServer()
}


func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
