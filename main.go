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

package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	policyv1beta1 "k8s.io/kubernetes/pkg/apis/policy/v1beta1"
	rbacv1 "k8s.io/kubernetes/pkg/apis/rbac/v1"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/controllers"
	"github.com/metallb/metallb-operator/pkg/params"
	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	// +kubebuilder:scaffold:imports
)

const (
	caName         = "cert"
	caOrganization = "metallb"
)

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs="*"

var (
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	webhookName       = "metallb-operator-webhook-configuration"
	webhookSecretName = "metallb-operator-webhook-server-cert"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(metallbv1beta1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(policyv1beta1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))
	utilruntime.Must(openshiftapiv1.AddToScheme(scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	var (
		metricsAddr          = flag.String("metrics-addr", ":0", "The address the metric endpoint binds to.")
		enableLeaderElection = flag.Bool("enable-leader-election", false, "Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
		disableCertRotation = flag.Bool("disable-cert-rotation", false, "disable automatic generation and rotation of webhook TLS certificates/keys")
		certDir             = flag.String("cert-dir", "/tmp/k8s-webhook-server/serving-certs", "The directory where certs are stored")
		certServiceName     = flag.String("cert-service-name", "metallb-operator-webhook-service", "The service name used to generate the TLS cert's hostname")
		port                = flag.Int("port", 8080, "HTTP listening port to check operator readiness")
		withWebhookHTTP2    = flag.Bool("webhook-http2", false, "enables http2 for the webhook endpoint")
	)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	setupLog.Info("git commit:", "id", build)

	cfg := ctrl.GetConfigOrDie()
	platformInfo, err := platform.GetPlatformInfo(cfg)
	if err != nil {
		setupLog.Error(err, "failed to get platform info")
		os.Exit(1)
	}
	envParams, err := params.FromEnvironment(platformInfo.IsOpenShift())
	if err != nil {
		setupLog.Error(err, "failed to parse env params")
		os.Exit(1)
	}
	jsonEnv, err := json.Marshal(envParams)
	if err != nil {
		setupLog.Error(err, "failed to marshal env params")
		os.Exit(1)
	}

	setupLog.Info("starting with env config ", "config", string(jsonEnv))

	namespaceSelector := cache.ByObject{
		Field: fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace=%s", envParams.Namespace)),
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: *metricsAddr,
		},

		LeaderElection:   *enableLeaderElection,
		LeaderElectionID: "metallb.io.metallboperator",
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&metallbv1beta1.MetalLB{}: namespaceSelector,
			},
		},
		WebhookServer: webhookServer(9443, *withWebhookHTTP2),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.MetalLBReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("MetalLB"),
		Scheme:    mgr.GetScheme(),
		Namespace: envParams.Namespace,
		EnvConfig: envParams,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetalLB")
		os.Exit(1)
	}

	setupFinished := make(chan struct{})
	go func() {
		// Block until the setup (certificate generation) finishes.
		setupLog.Info("waiting to create operator webhook for MetalLB CR")
		<-setupFinished
		setupLog.Info("creating operator webhook for MetalLB CR")
		if err = (&metallbv1beta1.MetalLB{}).SetupWebhookWithManager(mgr, envParams.FRRK8sExternalNamespace); err != nil {
			setupLog.Error(err, "unable to create webhook", "operator webhook", "MetalLB")
			os.Exit(1)
		}
		setupLog.Info("operator webhook for MetalLB CR is created")

		http.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
			_, err = w.Write([]byte("ok"))
			if err != nil {
				setupLog.Error(err, "error writing ok response", "readiness", "MetalLB")
			}
		})
		err = http.ListenAndServe(net.JoinHostPort("", fmt.Sprint(*port)), nil)
		if err != nil {
			setupLog.Error(err, "listenAndServe", "readiness", "MetalLB")
		}
	}()
	if !*disableCertRotation {
		setupLog.Info("setting up cert rotation for operator webhook")
		webhooks := []rotator.WebhookInfo{
			{
				Name: webhookName,
				Type: rotator.Validating,
			},
		}
		err = rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: envParams.Namespace,
				Name:      webhookSecretName,
			},
			CertDir:        *certDir,
			CAName:         caName,
			CAOrganization: caOrganization,
			DNSName:        fmt.Sprintf("%s.%s.svc", *certServiceName, envParams.Namespace),
			IsReady:        setupFinished,
			Webhooks:       webhooks,
		})
		if err != nil {
			setupLog.Error(err, "unable to setup cert rotation", "operator webhook", "MetalLB")
			os.Exit(1)
		}
		setupLog.Info("cert rotation setup for operator webhook is complete")
	} else {
		close(setupFinished)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func webhookServer(port int, withHTTP2 bool) webhook.Server {
	disableHTTP2 := func(c *tls.Config) {
		if withHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}

	webhookServerOptions := webhook.Options{
		TLSOpts: []func(config *tls.Config){disableHTTP2},
		Port:    port,
	}

	res := webhook.NewServer(webhookServerOptions)
	return res
}
