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

package helm

import (
	"fmt"
	"os"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bgpFRR = "frr"
)

// MetalLBChart metallb chart struct containing references which helps to
// to retrieve manifests from chart after patching given custom values.
type MetalLBChart struct {
	client      *action.Install
	envSettings *cli.EnvSettings
	chart       *chart.Chart
	config      *mlbChartConfig
	namespace   string
}

type mlbChartConfig struct {
	namespace            string
	isOpenShift          bool
	isFrrEnabled         bool
	controllerImage      *imageInfo
	speakerImage         *imageInfo
	frrImage             *imageInfo
	kubeRbacProxyImage   *imageInfo
	mlBindPort           int
	frrMetricsPort       int
	metricsPort          int
	secureMetricsPort    int
	secureFRRMetricsPort int
	enablePodMonitor     bool
	enableServiceMonitor bool
}

// GetObjects retrieve manifests from chart after patching custom values passed in crdConfig
// and environment variables.
func (h *MetalLBChart) GetObjects(crdConfig *metallbv1beta1.MetalLB, withPrometheus bool) ([]*unstructured.Unstructured, error) {
	chartValueOpts := &values.Options{}
	chartValues, err := chartValueOpts.MergeValues(getter.All(h.envSettings))
	if err != nil {
		return nil, err
	}

	h.config.patchChartValues(crdConfig, withPrometheus, chartValues)
	release, err := h.client.Run(h.chart, chartValues)
	if err != nil {
		return nil, err
	}
	objs, err := parseManifest(release.Manifest)
	if err != nil {
		return nil, err
	}
	for i, obj := range objs {
		// Set namespace explicitly into non cluster-scoped resource because helm doesn't
		// patch namespace into manifests at client.Run.
		objKind := obj.GetKind()
		if objKind != "PodSecurityPolicy" {
			obj.SetNamespace(h.namespace)
		}
		// patch affinity and resources parameters explicitly into appropriate obj.
		// This is needed because helm template doesn't support loading non table
		// structure values.
		objs[i], err = overrideControllerParameters(crdConfig, objs[i])
		if err != nil {
			return nil, err
		}
		objs[i], err = overrideSpeakerParameters(crdConfig, objs[i])
		if err != nil {
			return nil, err
		}
		// we need to override the security context as helm values are added on top
		// of hardcoded ones in values.yaml, so it's not possible to reset runAsUser
		if isControllerDeployment(obj) && h.config.isOpenShift {
			controllerSecurityContext := map[string]interface{}{
				"runAsNonRoot": true,
			}
			err := unstructured.SetNestedMap(obj.Object, controllerSecurityContext, "spec", "template", "spec", "securityContext")
			if err != nil {
				return nil, err
			}
		}
		if isServiceMonitor(obj) && h.config.isOpenShift {
			err := setOcpMonitorFields(obj)
			if err != nil {
				return nil, err
			}
		}
	}
	return objs, nil
}

// NewMetalLBChart initializes metallb helm chart after loading it from given
// chart path and creating config object from environment variables.
func NewMetalLBChart(chartPath, chartName, namespace string,
	client client.Client, isOpenshift bool) (*MetalLBChart, error) {
	chart := &MetalLBChart{}
	chart.namespace = namespace
	chart.envSettings = cli.New()
	chart.client = action.NewInstall(new(action.Configuration))
	chart.client.ReleaseName = chartName
	chart.client.DryRun = true
	chart.client.ClientOnly = true
	chart.client.Namespace = namespace
	chartPath, err := chart.client.ChartPathOptions.LocateChart(chartPath, chart.envSettings)
	if err != nil {
		return nil, err
	}
	chart.chart, err = loader.Load(chartPath)
	if err != nil {
		return nil, err
	}
	chart.config, err = loadMetalLBConfig(namespace, isOpenshift)
	if err != nil {
		return nil, err
	}
	return chart, nil
}

func overrideControllerParameters(crdConfig *metallbv1beta1.MetalLB, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	controllerConfig := crdConfig.Spec.ControllerConfig
	if controllerConfig == nil || !isControllerDeployment(obj) {
		return obj, nil
	}
	var controller *appsv1.Deployment
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &controller)
	if err != nil {
		return nil, err
	}
	if controllerConfig.Affinity != nil {
		controller.Spec.Template.Spec.Affinity = controllerConfig.Affinity
	}
	for j, container := range controller.Spec.Template.Spec.Containers {
		if container.Name == "controller" && controllerConfig.Resources != nil {
			controller.Spec.Template.Spec.Containers[j].Resources = *controllerConfig.Resources
		}
	}
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(controller)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: objMap}, nil
}

func overrideSpeakerParameters(crdConfig *metallbv1beta1.MetalLB, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	speakerConfig := crdConfig.Spec.SpeakerConfig
	if speakerConfig == nil || !isSpeakerDaemonSet(obj) {
		return obj, nil
	}
	var speaker *appsv1.DaemonSet
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &speaker)
	if err != nil {
		return nil, err
	}
	if speakerConfig.Affinity != nil {
		speaker.Spec.Template.Spec.Affinity = speakerConfig.Affinity
	}
	for j, container := range speaker.Spec.Template.Spec.Containers {
		if container.Name == "speaker" && speakerConfig.Resources != nil {
			speaker.Spec.Template.Spec.Containers[j].Resources = *speakerConfig.Resources
		}
	}
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(speaker)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: objMap}, nil
}

func isControllerDeployment(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Deployment" && obj.GetName() == "controller"
}

func isSpeakerDaemonSet(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "DaemonSet" && obj.GetName() == "speaker"
}

func isServiceMonitor(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "ServiceMonitor"
}

func (c *mlbChartConfig) patchChartValues(crdConfig *metallbv1beta1.MetalLB, withPrometheus bool, valuesMap map[string]interface{}) {
	valuesMap["loadBalancerClass"] = loadBalancerClassValue(crdConfig)
	valuesMap["prometheus"] = c.prometheusValues()
	valuesMap["controller"] = c.controllerValues(crdConfig)
	valuesMap["speaker"] = c.speakerValues(crdConfig)
}

func loadBalancerClassValue(crdConfig *metallbv1beta1.MetalLB) string {
	return crdConfig.Spec.LoadBalancerClass
}

func (c *mlbChartConfig) prometheusValues() map[string]interface{} {
	speakerTLSConfig := map[string]interface{}{
		"insecureSkipVerify": true,
	}
	controllerTLSConfig := map[string]interface{}{
		"insecureSkipVerify": true,
	}
	speakerAnnotations := map[string]interface{}{}
	controllerAnnotations := map[string]interface{}{}

	speakerTLSSecret := ""
	controllerTLSSecret := ""

	if c.isOpenShift {
		speakerTLSConfig, speakerAnnotations, speakerTLSSecret = ocpPromConfigFor("speaker", c.namespace)
		controllerTLSConfig, controllerAnnotations, controllerTLSSecret = ocpPromConfigFor("controller", c.namespace)
	}

	return map[string]interface{}{
		"metricsPort":       c.metricsPort,
		"secureMetricsPort": c.secureMetricsPort,
		"podMonitor": map[string]interface{}{
			"enabled": c.enablePodMonitor,
		},
		"serviceMonitor": map[string]interface{}{
			"enabled": c.enableServiceMonitor,
			"speaker": map[string]interface{}{
				"annotations": speakerAnnotations,
				"tlsConfig":   speakerTLSConfig,
			},
			"controller": map[string]interface{}{
				"annotations": controllerAnnotations,
				"tlsConfig":   controllerTLSConfig,
			},
		},
		"rbacProxy": map[string]interface{}{
			"repository": c.kubeRbacProxyImage.repo,
			"tag":        c.kubeRbacProxyImage.tag,
		},
		"serviceAccount":             "foo", // required by the chart, we won't render roles or rolebindings anyway
		"namespace":                  "bar",
		"speakerMetricsTLSSecret":    speakerTLSSecret,
		"controllerMetricsTLSSecret": controllerTLSSecret,
	}
}

func (c *mlbChartConfig) controllerValues(crdConfig *metallbv1beta1.MetalLB) map[string]interface{} {
	controllerValueMap := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": c.controllerImage.repo,
			"tag":        c.controllerImage.tag,
		},
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "controller",
		},
		"webhookMode": "disabled",
	}
	controllerValueMap["logLevel"] = logLevelValue(crdConfig)
	if c.isOpenShift {
		controllerValueMap["securityContext"] = map[string]interface{}{
			"runAsNonRoot": true,
			"runAsUser":    nil,
			"fsGroup":      nil,
		}
		controllerValueMap["command"] = "/controller"
	}
	if crdConfig.Spec.ControllerNodeSelector != nil {
		controllerValueMap["nodeSelector"] = toInterfaceMap(crdConfig.Spec.ControllerNodeSelector)
	}
	if crdConfig.Spec.ControllerTolerations != nil {
		controllerValueMap["tolerations"] = crdConfig.Spec.ControllerTolerations
	}
	otherConfigs := crdConfig.Spec.ControllerConfig
	if otherConfigs != nil {
		if otherConfigs.PriorityClassName != "" {
			controllerValueMap["priorityClassName"] = otherConfigs.PriorityClassName
		}
		if otherConfigs.RuntimeClassName != "" {
			controllerValueMap["runtimeClassName"] = otherConfigs.RuntimeClassName
		}
		if otherConfigs.Annotations != nil {
			controllerValueMap["podAnnotations"] = toInterfaceMap(otherConfigs.Annotations)
		}
	}
	return controllerValueMap
}

func (c *mlbChartConfig) speakerValues(crdConfig *metallbv1beta1.MetalLB) map[string]interface{} {
	speakerValueMap := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": c.speakerImage.repo,
			"tag":        c.speakerImage.tag,
		},
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "speaker",
		},
		"frr": map[string]interface{}{
			"enabled": c.isFrrEnabled,
			"image": map[string]interface{}{
				"repository": c.frrImage.repo,
				"tag":        c.frrImage.tag,
			},
			"metricsPort":       c.frrMetricsPort,
			"secureMetricsPort": c.secureFRRMetricsPort,
		},
		"memberlist": map[string]interface{}{
			"enabled":    true,
			"mlBindPort": c.mlBindPort,
		},
		"command": "/speaker",
	}
	speakerValueMap["logLevel"] = logLevelValue(crdConfig)
	if crdConfig.Spec.SpeakerNodeSelector != nil {
		speakerValueMap["nodeSelector"] = toInterfaceMap(crdConfig.Spec.SpeakerNodeSelector)
	}
	if crdConfig.Spec.SpeakerTolerations != nil {
		speakerValueMap["tolerations"] = crdConfig.Spec.SpeakerTolerations
	}
	otherConfigs := crdConfig.Spec.SpeakerConfig
	if otherConfigs != nil {
		if otherConfigs.PriorityClassName != "" {
			speakerValueMap["priorityClassName"] = otherConfigs.PriorityClassName
		}
		if otherConfigs.RuntimeClassName != "" {
			speakerValueMap["runtimeClassName"] = otherConfigs.RuntimeClassName
		}
		if otherConfigs.Annotations != nil {
			speakerValueMap["podAnnotations"] = toInterfaceMap(otherConfigs.Annotations)
		}
	}
	return speakerValueMap
}

func loadMetalLBConfig(namespace string, isOCP bool) (*mlbChartConfig, error) {
	config := &mlbChartConfig{
		isOpenShift:        isOCP,
		namespace:          namespace,
		kubeRbacProxyImage: &imageInfo{},
		frrImage:           &imageInfo{},
	}
	var err error
	ctrlImage := os.Getenv("CONTROLLER_IMAGE")
	if ctrlImage == "" {
		return nil, errors.Errorf("CONTROLLER_IMAGE env variable must be set")
	}
	controllerRepo, controllerTag := getImageNameTag(ctrlImage)
	config.controllerImage = &imageInfo{controllerRepo, controllerTag}
	speakerImage := os.Getenv("SPEAKER_IMAGE")
	if speakerImage == "" {
		return nil, errors.Errorf("SPEAKER_IMAGE env variable must be set")
	}
	speakerRepo, speakerTag := getImageNameTag(speakerImage)
	config.speakerImage = &imageInfo{speakerRepo, speakerTag}
	if os.Getenv("METALLB_BGP_TYPE") == bgpFRR {
		config.isFrrEnabled = true
		frrImage := os.Getenv("FRR_IMAGE")
		if frrImage == "" {
			return nil, errors.Errorf("FRR_IMAGE env variable must be set")
		}
		config.frrImage.repo, config.frrImage.tag = getImageNameTag(frrImage)
	}
	config.mlBindPort, err = valueWithDefault("MEMBER_LIST_BIND_PORT", 7946)
	if err != nil {
		return nil, err
	}
	config.frrMetricsPort, err = valueWithDefault("FRR_METRICS_PORT", 7473)
	if err != nil {
		return nil, err
	}
	config.secureFRRMetricsPort, err = valueWithDefault("FRR_HTTPS_METRICS_PORT", 0)
	if err != nil {
		return nil, err
	}
	config.metricsPort, err = valueWithDefault("METRICS_PORT", 7472)
	if err != nil {
		return nil, err
	}
	config.secureMetricsPort, err = valueWithDefault("HTTPS_METRICS_PORT", 0)
	if err != nil {
		return nil, err
	}
	// We shouldn't spam the api server trying to apply PodMonitors if the resource isn't installed.
	if os.Getenv("DEPLOY_PODMONITORS") == "true" {
		config.enablePodMonitor = true
	}
	// We shouldn't spam the api server trying to apply PodMonitors if the resource isn't installed.
	if os.Getenv("DEPLOY_SERVICEMONITORS") == "true" {
		config.enableServiceMonitor = true
	}

	kubeRbacProxyImage := os.Getenv("KUBE_RBAC_PROXY_IMAGE")
	if kubeRbacProxyImage == "" {
		return nil, errors.Errorf("KUBE_RBAC_PROXY_IMAGE env variable must be set")
	}
	config.kubeRbacProxyImage.repo, config.kubeRbacProxyImage.tag = getImageNameTag(kubeRbacProxyImage)
	err = validateMetalLBConfig(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func validateMetalLBConfig(c *mlbChartConfig) error {
	if c.enablePodMonitor && c.enableServiceMonitor {
		return fmt.Errorf("pod monitors and service monitors are mutually exclusive, only one can be enabled")
	}
	if c.secureMetricsPort != 0 && !c.enableServiceMonitor {
		return fmt.Errorf("secureMetricsPort is available only if service monitors are enabled")
	}
	if c.secureFRRMetricsPort != 0 && !c.enableServiceMonitor {
		return fmt.Errorf("secureFRRMetricsPort is available only if service monitors are enabled")
	}
	if c.isOpenShift && !c.enableServiceMonitor {
		return fmt.Errorf("service monitors are required on OpenShift")
	}
	return nil
}
