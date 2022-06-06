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
	"context"
	"encoding/json"
	"flag"
	"os"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	policyv1beta1 "k8s.io/kubernetes/pkg/apis/policy/v1beta1"
	rbacv1 "k8s.io/kubernetes/pkg/apis/rbac/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/controllers"
	"github.com/metallb/metallb-operator/pkg/platform"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(metallbv1beta1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(policyv1beta1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	setupLog.Info("git commit:", "id", build)

	watchNamespace := checkEnvVar("WATCH_NAMESPACE")
	checkEnvVar("SPEAKER_IMAGE")
	checkEnvVar("CONTROLLER_IMAGE")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "metallb.io.metallboperator",
		Namespace:          watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg := ctrl.GetConfigOrDie()
	platformInfo, err := platform.GetPlatformInfo(cfg)
	if err != nil {
		setupLog.Error(err, "unable to get platform name")
		os.Exit(1)
	}

	webHookEnabled, err := strconv.ParseBool(os.Getenv("ENABLE_WEBHOOK"))
	if err != nil {
		setupLog.Error(err, "unable to get enable webhook parameter")
		os.Exit(1)
	}

	bgpType := os.Getenv("METALLB_BGP_TYPE")
	if err = (&controllers.MetalLBReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("MetalLB"),
		Scheme:       mgr.GetScheme(),
		PlatformInfo: platformInfo,
		Namespace:    watchNamespace,
	}).SetupWithManager(mgr, bgpType, webHookEnabled); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetalLB")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	err = createMetallb(watchNamespace, os.Getenv("SPEAKER_NODE_SELECTOR"), mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "failed to create metallb")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func checkEnvVar(name string) string {
	value, isSet := os.LookupEnv(name)
	if !isSet {
		setupLog.Error(nil, "env variable must be set", "name", name)
		os.Exit(1)
	}
	return value
}

func createMetallb(namespace, selector string, c client.Client) error {
	instance := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      controllers.DefaultMetalLBCrName,
		},
	}

	if selector != "" {
		err := json.Unmarshal([]byte(selector), &instance.Spec.SpeakerNodeSelector)
		if err != nil {
			setupLog.Error(err, "failed to parse node selector")
			return err
		}
	}
	err := c.Create(context.TODO(), instance)
	if k8serrors.IsAlreadyExists(err) {
		setupLog.Info("metallb already exists, not creating")
		return nil
	}
	if err != nil {
		return err
	}
	setupLog.Info("created metallb resource")
	return nil
}
