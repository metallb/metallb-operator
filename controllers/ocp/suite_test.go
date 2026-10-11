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

// Package ocp holds the controller tests that need the operator to run in openshift mode.
// The mode is set from the environment when the operator starts and never changes afterwards,
// so it is given its own suite rather than being flipped on a running manager.
package ocp

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/controllers"
	"github.com/metallb/metallb-operator/pkg/params"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftoperatorv1 "github.com/openshift/api/operator/v1"
)

const (
	MetalLBHelmChartPathTest = "../../bindata/deployment/helm/metallb"
	FRRK8SHelmChartPathTest  = "../../bindata/deployment/helm/frr-k8s"
	MetalLBTestNameSpace     = "metallb-test-namespace"
	FRRK8SExternalNameSpace  = "openshift-frr-k8s"
	CNOMinFRRK8sVersion      = "4.17.0-0"
	// Shortened so the tests do not have to wait for the production interval to observe
	// the operator recovering once the CRD catches up.
	CNOHandoffRequeueAfterTest = 500 * time.Millisecond
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	managerWG sync.WaitGroup
)

func TestOpenshiftAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Openshift Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "hack", "openshiftapicrds"),
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
	err = apiextensionsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		// Avoid binding the default metrics port, it collides with the controllers suite
		// when go test runs the two packages in parallel.
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).ToNot(HaveOccurred())

	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: MetalLBTestNameSpace,
		},
	}
	err = k8sClient.Create(ctx, testNamespace)
	Expect(err).ToNot(HaveOccurred())

	// The charts are referenced from the controllers package, so the paths need to be
	// adjusted to this package's directory.
	controllers.MetalLBChartPath = MetalLBHelmChartPathTest
	controllers.FRRK8SChartPath = FRRK8SHelmChartPathTest
	controllers.CNOHandoffRequeueAfter = CNOHandoffRequeueAfterTest

	reconciler := &controllers.MetalLBReconciler{
		Client:    k8sClient,
		Scheme:    scheme.Scheme,
		Log:       ctrl.Log.WithName("controllers").WithName("MetalLB"),
		Namespace: MetalLBTestNameSpace,
		EnvConfig: envConfig,
	}
	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	managerWG.Go(func() {
		defer GinkgoRecover()
		err := k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	managerWG.Wait()
	controllers.MetalLBChartPath = controllers.MetalLBChartPathController
	controllers.FRRK8SChartPath = controllers.FRRK8SChartPathController
	controllers.CNOHandoffRequeueAfter = controllers.CNOHandoffRequeueAfterController
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

var envConfig = params.EnvConfig{
	IsOpenshift:             true,
	MustDeployFRRK8sFromCNO: true,
	FRRK8sExternalNamespace: FRRK8SExternalNameSpace,
	CNOMinFRRK8sVersion:     CNOMinFRRK8sVersion,
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
	FRRK8sImage: params.ImageInfo{
		Repo: "frr-k8s",
		Tag:  "test",
	},
	Namespace:                  MetalLBTestNameSpace,
	MetricsPort:                9120,
	LivenessPort:               17472,
	FRRMetricsPort:             9121,
	MLBindPort:                 7946,
	FRRK8sMetricsPort:          7572,
	SecureFRRK8sMetricsPort:    9140,
	FRRK8sFRRMetricsPort:       7573,
	SecureFRRK8sFRRMetricsPort: 9141,
}
