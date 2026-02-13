/*


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

package controllers

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/params"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftoperatorv1 "github.com/openshift/api/operator/v1"
	// +kubebuilder:scaffold:imports
)

const (
	MetalLBHelmChartPathControllerTest = "../bindata/deployment/helm/metallb"
	FRRK8SHelmChartPathControllerTest  = "../bindata/deployment/helm/frr-k8s"
	MetalLBTestNameSpace               = "metallb-test-namespace"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg        *rest.Config
	k8sClient  client.Client
	testEnv    *envtest.Environment
	reconciler *MetalLBReconciler
	ctx        context.Context
	cancel     context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	_, reporterConfig := GinkgoConfiguration()

	RunSpecs(t, "Controller Suite", reporterConfig)
}

var defaultEnvConfig = params.EnvConfig{
	SpeakerImage: params.ImageInfo{
		Repo: "test-speaker",
		Tag:  "latest",
	},
	ControllerImage: params.ImageInfo{
		Repo: "test-controller",
		Tag:  "latest",
	},
	FRRImage: params.ImageInfo{
		Repo: "test-frr",
		Tag:  "latest",
	},
	KubeRBacImage: params.ImageInfo{
		Repo: "test-kube-rbac-proxy",
		Tag:  "latest",
	},
	FRRK8sImage: params.ImageInfo{
		Repo: "frr-k8s",
		Tag:  "test",
	},
	Namespace:                  MetalLBTestNameSpace,
	MetricsPort:                7472,
	FRRMetricsPort:             7473,
	MLBindPort:                 7946,
	FRRK8sMetricsPort:          7572,
	SecureFRRK8sMetricsPort:    9140,
	FRRK8sFRRMetricsPort:       7573,
	SecureFRRK8sFRRMetricsPort: 9141,
}
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("Setting MetalLBReconcilier environment variables")

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "hack", "openshiftapicrds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = metallbv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = openshiftoperatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = openshiftconfigv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: MetalLBTestNameSpace,
		},
	}

	err = k8sClient.Create(ctx, testNamespace)
	Expect(err).ToNot(HaveOccurred())

	MetalLBChartPath = MetalLBHelmChartPathControllerTest // This is needed as the tests need to reference a directory backward
	FRRK8SChartPath = FRRK8SHelmChartPathControllerTest

	reconciler = &MetalLBReconciler{
		Client:    k8sClient,
		Scheme:    scheme.Scheme,
		Log:       ctrl.Log.WithName("controllers").WithName("MetalLB"),
		Namespace: MetalLBTestNameSpace,
		EnvConfig: defaultEnvConfig,
	}
	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	// restore Manifestpaths for both controller to their original value
	MetalLBChartPath = MetalLBChartPathController
	FRRK8SChartPath = FRRK8SChartPathController
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
