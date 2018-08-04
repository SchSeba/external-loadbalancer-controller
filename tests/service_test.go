package tests_test

import (
	. "github.com/onsi/ginkgo"
	"github.com/k8s-external-lb/external-loadbalancer-controller/tests"
	"github.com/k8s-external-lb/external-loadbalancer-controller/pkg/apis/manager/v1alpha1"
	//k8sv1 "k8s.io/api/core/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sv1 "k8s.io/api/core/v1"
)

var _ = Describe("Service", func() {

	kubeCli := tests.GetKubeClient()
	restClient := tests.GetKubeRestClient()
	provider := &v1alpha1.Provider{ObjectMeta:metav1.ObjectMeta{Name:"provider-sample",Namespace:v1alpha1.ControllerNamespace},TypeMeta: metav1.TypeMeta{Kind:"Provider",APIVersion:"manager.external-loadbalancer/v1alpha1"},Spec:v1alpha1.ProviderSpec{Url:"127.0.0.1",Default:true}}

	BeforeEach(func() {
		go tests.StartTestGrpcServer()
		result := &v1alpha1.Provider{}
		err := restClient.Post().Resource("providers").Namespace(v1alpha1.ControllerNamespace).Name("provider-sample").Body(provider).Do().Into(result)
		Expect(err).ToNot(HaveOccurred())
		//time.Sleep(5 * time.Second)
	})

	AfterEach(func() {
		tests.StopTestGrpcServer()
		err := restClient.Delete().Resource("providers").Namespace(v1alpha1.ControllerNamespace).Name("provider-sample").Body(&metav1.DeleteOptions{}).Do().Error()
		Expect(err).ToNot(HaveOccurred())

	})

	Describe("Create a new Service", func() {
		It("should successfully create a farm with the service properties", func() {
			_, err := kubeCli.CoreV1().Services(tests.NamespaceTestDefault).Create(&k8sv1.Service{ObjectMeta:metav1.ObjectMeta{Name:"test-service",Namespace:tests.NamespaceTestDefault},
																						Spec:k8sv1.ServiceSpec{Type:"LoadBalancer",Ports:[]k8sv1.ServicePort{{Port:80}}}})
			Expect(err).ToNot(HaveOccurred())

		})
	})

})
