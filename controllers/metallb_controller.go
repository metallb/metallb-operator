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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/metallb/metallb-operator/pkg/status"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

const (
	defaultMetalLBCrName          = "metallb"
	MetalLBManifestPathController = "./bindata/deployment"
	MetalLBSpeakerDaemonSet       = "speaker"
	MetalLBWebhookSecret          = "webhook-server-cert"
)

const (
	helmReleaseName                   = "metallb"
	bgpNative                  string = "native"
	bgpFrr                     string = "frr"
	defaultControllerImageRepo        = "quay.io/metallb/controller"
	defaultSpeakerImageRepo           = "quay.io/metallb/speaker"
	defaultFRRImageRepo               = "frrouting/frr"
	defaultFRRImageTag                = "v7.5.1"
)

// MetalLBReconciler reconciles a MetalLB object
type MetalLBReconciler struct {
	client.Client
	Settings     *cli.EnvSettings
	KubeConfig   *genericclioptions.ConfigFlags
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PlatformInfo platform.PlatformInfo
	Namespace    string
}

var ManifestPath = MetalLBManifestPathController
var PodMonitorsPath = fmt.Sprintf("%s/%s", MetalLBManifestPathController, "prometheus-operator")

// Namespace Scoped
// +kubebuilder:rbac:groups=apps,namespace=metallb-system,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=metallb-system,resources=services,verbs=create;delete;get;update;patch

// Cluster Scoped
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=policy,resources=podsecuritypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/finalizers,verbs=delete;get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;delete;get;update;patch;list;watch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=create;delete;get;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings;rolebindings;clusterroles;roles,verbs=bind;create;delete;escalate;get;update;patch;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;serviceaccounts,verbs=create;delete;get;update;patch;list;watch

func (r *MetalLBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("metallb", req.NamespacedName)

	instance := &metallbv1beta1.MetalLB{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err := r.syncMetalLBResources(nil, true)
			if err != nil {
				logger.Error(err, "error while syncing metallb")
			}
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if req.Name != defaultMetalLBCrName {
		err := fmt.Errorf("MetalLB resource name must be '%s'", defaultMetalLBCrName)
		logger.Error(err, "Invalid MetalLB resource name", "name", req.Name)
		if err := status.Update(context.TODO(), r.Client, instance, status.ConditionDegraded, fmt.Sprintf("Incorrect MetalLB resource name: %s", req.Name)); err != nil {
			logger.Error(err, "Failed to update metallb status", "Desired status", status.ConditionDegraded)
		}
		return ctrl.Result{}, nil // Return success to avoid requeue
	}

	result, condition, err := r.reconcileResource(ctx, req, instance)
	if condition != "" {
		errorMsg := ""
		if err != nil {
			if errors.Unwrap(err) != nil {
				errorMsg = errors.Unwrap(err).Error()
			}
		}
		if err := status.Update(context.TODO(), r.Client, instance, condition, errorMsg); err != nil {
			logger.Error(err, "Failed to update metallb status", "Desired status", status.ConditionAvailable)
		}
	}
	return result, err
}

func (r *MetalLBReconciler) reconcileResource(ctx context.Context, req ctrl.Request, instance *metallbv1beta1.MetalLB) (ctrl.Result, string, error) {
	err := r.syncMetalLBResources(instance, false)
	if err != nil {
		return ctrl.Result{}, status.ConditionDegraded, errors.Wrapf(err, "FailedToSyncMetalLBResources")
	}
	err = status.IsMetalLBAvailable(context.TODO(), r.Client, req.NamespacedName.Namespace)
	if err != nil {
		if _, ok := err.(status.MetalLBResourcesNotReadyError); ok {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, status.ConditionProgressing, nil
		}
		return ctrl.Result{}, status.ConditionProgressing, err
	}
	return ctrl.Result{}, status.ConditionAvailable, nil
}

func (r *MetalLBReconciler) SetupWithManager(mgr ctrl.Manager, bgpType string) error {
	if bgpType == "" {
		bgpType = bgpNative
	}
	if bgpType != bgpNative && bgpType != bgpFrr {
		return fmt.Errorf("unsupported BGP implementation type: %s", bgpType)
	}
	ManifestPath = fmt.Sprintf("%s/helm", ManifestPath)
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta1.MetalLB{}).
		Complete(r)
}

func (r *MetalLBReconciler) syncMetalLBResources(config *metallbv1beta1.MetalLB, isUninstall bool) error {
	logger := r.Log.WithName("syncMetalLBResources")
	logger.Info("Start")

	actionConfig := new(action.Configuration)
	debugLog := func(format string, v ...interface{}) {
		logger.Info("reconciler", "helm", fmt.Sprintf(format, v...))
	}
	kubeConfig := r.Settings.RESTClientGetter()
	if r.KubeConfig != nil {
		kubeConfig = r.KubeConfig
	}
	if err := actionConfig.Init(kubeConfig, r.Settings.Namespace(), os.Getenv("HELM_DRIVER"), debugLog); err != nil {
		return err
	}

	listClient := action.NewList(actionConfig)
	results, err := listClient.Run()
	var isMetalLBInstalled bool
	for _, rel := range results {
		logger.Info("reconciler", "existing helm chart", rel.Name)
		if rel.Name == helmReleaseName {
			isMetalLBInstalled = true
			break
		}
	}
	if isUninstall {
		if !isMetalLBInstalled {
			logger.Info("reconciler", "chart already uninstalled", helmReleaseName)
			return nil
		}
		logger.Info("reconciler", "chart uninstalling", helmReleaseName)
		uninstallClient := action.NewUninstall(actionConfig)
		release, err := uninstallClient.Run(helmReleaseName)
		if err != nil {
			return err
		}
		logger.Info("reconciler", "uinstalled chart", release.Release.Name)
		return nil
	}
	chartValueOpts := &values.Options{}
	chartValues, err := chartValueOpts.MergeValues(getter.All(r.Settings))
	if err != nil {
		return err
	}
	err = patchPrometheusValues(chartValues, r.Client)
	if err != nil {
		return err
	}
	patchControllerValues(chartValues, config, r.PlatformInfo.IsOpenShift())
	err = patchSpeakerValues(chartValues, config)
	if err != nil {
		return err
	}
	// TODO: patch rbac proxy settings if needed.
	if isMetalLBInstalled {
		logger.Info("reconciler", "chart upgrading", helmReleaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = r.Settings.Namespace()
		chartPath, err := upgradeClient.ChartPathOptions.LocateChart(ManifestPath, r.Settings)
		if err != nil {
			return err
		}
		chart, err := loader.Load(chartPath)
		if err != nil {
			return err
		}
		release, err := upgradeClient.Run(helmReleaseName, chart, chartValues)
		if err != nil {
			return err
		}
		logger.Info("reconciler", "upgraded chart", release.Name)
		return nil
	}

	client := action.NewInstall(actionConfig)
	client.ReleaseName = "metallb"
	client.Namespace = r.Settings.Namespace()
	client.Verify = false
	chartPath, err := client.ChartPathOptions.LocateChart(ManifestPath, r.Settings)
	if err != nil {
		return err
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		logger.Error(err, "chart loading failed")
		return err
	}
	logger.Info("reconciler", "chart installing", helmReleaseName, "path", chartPath, "chart name", chart.Name())
	release, err := client.Run(chart, chartValues)
	if err != nil {
		logger.Error(err, "chart install failed")
		return err
	}
	logger.Info("reconciler", "installed chart", release.Name)
	return nil
}

func patchPrometheusValues(chartValues map[string]interface{}, client client.Client) error {
	metricsPort, err := valueWithDefault("METRICS_PORT", 7472)
	if err != nil {
		return err
	}
	// We shouldn't spam the api server trying to apply PodMonitors if the resource isn't installed.
	var enablePodMonitor bool
	if podMonitorAvailable(client) && os.Getenv("DEPLOY_PODMONITORS") == "true" {
		enablePodMonitor = true
	}
	chartValues["prometheus"] = map[string]interface{}{
		"metricsPort": metricsPort,
		"podMonitor": map[string]interface{}{
			"enabled": enablePodMonitor,
		},
	}
	return nil
}

func patchControllerValues(chartValues map[string]interface{}, config *metallbv1beta1.MetalLB, isOpenShift bool) {
	logLevel := metallbv1beta1.LogLevelInfo
	if config.Spec.LogLevel != "" {
		logLevel = config.Spec.LogLevel
	}

	ctrlImage := os.Getenv("CONTROLLER_IMAGE")
	controllerRepo, controllerTag := getImageNameTag(ctrlImage)
	if controllerRepo == "" {
		controllerRepo = defaultControllerImageRepo
	}
	if isOpenShift {
		chartValues["controller"] = map[string]interface{}{
			"image": map[string]interface{}{
				"repository": controllerRepo,
				"tag":        controllerTag,
			},
			"logLevel": logLevel,
			"securityContext": map[string]interface{}{
				"runAsNonRoot": true,
			},
		}
		return
	}
	chartValues["controller"] = map[string]interface{}{
		"image": map[string]interface{}{
			"repository": controllerRepo,
			"tag":        controllerTag,
		},
		"logLevel": logLevel,
	}
}

func patchSpeakerValues(chartValues map[string]interface{}, config *metallbv1beta1.MetalLB) error {
	logLevel := metallbv1beta1.LogLevelInfo
	if config.Spec.LogLevel != "" {
		logLevel = config.Spec.LogLevel
	}
	speakerImage := os.Getenv("SPEAKER_IMAGE")
	speakerRepo, speakerTag := getImageNameTag(speakerImage)
	if speakerRepo == "" {
		speakerRepo = defaultSpeakerImageRepo
	}
	frrEnabled := false
	var frrRepo, frrTag string
	if os.Getenv("METALLB_BGP_TYPE") == bgpFrr {
		frrEnabled = true
		frrRepo, frrTag = getImageNameTag(os.Getenv("FRR_IMAGE"))
		if frrRepo == "" {
			frrRepo = defaultFRRImageRepo
			frrTag = defaultFRRImageTag
		}
	}
	mlBindPort, err := valueWithDefault("MEMBER_LIST_BIND_PORT", 7946)
	if err != nil {
		return err
	}
	frrMetricsPort, err := valueWithDefault("FRR_METRICS_PORT", 7473)
	if err != nil {
		return err
	}
	chartValues["speaker"] = map[string]interface{}{
		"image": map[string]interface{}{
			"repository": speakerRepo,
			"tag":        speakerTag,
		},
		"frr": map[string]interface{}{
			"enabled": frrEnabled,
			"image": map[string]interface{}{
				"repository": frrRepo,
				"tag":        frrTag,
			},
			"metricsPort": frrMetricsPort,
		},
		"memberlist": map[string]interface{}{
			"enabled":    true,
			"mlBindPort": mlBindPort,
		},
		"logLevel": logLevel,
	}
	if config.Spec.SpeakerNodeSelector != nil {
		chartValues["speaker"].(map[string]interface{})["nodeSelector"] = config.Spec.SpeakerNodeSelector
	}
	if config.Spec.SpeakerTolerations != nil {
		chartValues["speaker"].(map[string]interface{})["tolerations"] = config.Spec.SpeakerTolerations
	}
	return nil
}

func podMonitorAvailable(c client.Client) bool {
	crd := &apiext.CustomResourceDefinition{}
	err := c.Get(context.Background(), client.ObjectKey{Name: "podmonitors.monitoring.coreos.com"}, crd)
	return err == nil
}

func valueWithDefault(name string, def int) (int, error) {
	val := os.Getenv(name)
	if val != "" {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return res, nil
	}
	return def, nil
}

func getImageNameTag(envString string) (string, string) {
	img := strings.Split(envString, ":")
	if len(img) == 0 {
		return "", ""
	} else if len(img) == 1 {
		return strings.TrimSpace(img[0]), ""
	}
	return strings.TrimSpace(img[0]), strings.TrimSpace(img[1])
}
