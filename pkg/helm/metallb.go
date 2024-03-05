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
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/params"
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

// MetalLBChart metallb chart struct containing references which helps to
// to retrieve manifests from chart after patching given custom values.
type MetalLBChart struct {
	client      *action.Install
	envSettings *cli.EnvSettings
	chart       *chart.Chart
}

// Objects retrieves manifests from chart after patching custom values passed in crdConfig
// and environment variables.
func (h *MetalLBChart) Objects(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB) ([]*unstructured.Unstructured, error) {
	chartValueOpts := &values.Options{}
	chartValues, err := chartValueOpts.MergeValues(getter.All(h.envSettings))
	if err != nil {
		return nil, err
	}

	patchMetalLBChartValues(envConfig, crdConfig, chartValues)
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
			obj.SetNamespace(envConfig.Namespace)
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
		if isControllerDeployment(obj) && envConfig.IsOpenshift {
			controllerSecurityContext := map[string]interface{}{
				"runAsNonRoot": true,
			}
			err := unstructured.SetNestedMap(obj.Object, controllerSecurityContext, "spec", "template", "spec", "securityContext")
			if err != nil {
				return nil, err
			}
		}
		if isServiceMonitor(obj) && envConfig.IsOpenshift {
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
	client client.Client) (*MetalLBChart, error) {
	chart := &MetalLBChart{}
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

func patchMetalLBChartValues(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB, valuesMap map[string]interface{}) {
	valuesMap["loadBalancerClass"] = loadBalancerClassValue(crdConfig)
	valuesMap["prometheus"] = metalLBprometheusValues(envConfig)
	valuesMap["controller"] = controllerValues(envConfig, crdConfig)
	valuesMap["speaker"] = speakerValues(envConfig, crdConfig)
	valuesMap["frrk8s"] = metalLBFrrk8sValues(envConfig)
}

func loadBalancerClassValue(crdConfig *metallbv1beta1.MetalLB) string {
	return crdConfig.Spec.LoadBalancerClass
}

func metalLBprometheusValues(envConfig params.EnvConfig) map[string]interface{} {
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

	if envConfig.IsOpenshift {
		speakerTLSConfig, speakerAnnotations, speakerTLSSecret = ocpPromConfigFor("speaker", envConfig.Namespace)
		controllerTLSConfig, controllerAnnotations, controllerTLSSecret = ocpPromConfigFor("controller", envConfig.Namespace)
	}

	return map[string]interface{}{
		"metricsPort":       envConfig.MetricsPort,
		"secureMetricsPort": envConfig.SecureMetricsPort,
		"podMonitor": map[string]interface{}{
			"enabled": envConfig.DeployPodMonitors,
		},
		"serviceMonitor": map[string]interface{}{
			"enabled": envConfig.DeployServiceMonitors,
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
			"repository": envConfig.KubeRBacImage.Repo,
			"tag":        envConfig.KubeRBacImage.Tag,
		},
		"serviceAccount":             "foo", // required by the chart, we won't render roles or rolebindings anyway
		"namespace":                  "bar",
		"speakerMetricsTLSSecret":    speakerTLSSecret,
		"controllerMetricsTLSSecret": controllerTLSSecret,
	}
}

func controllerValues(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB) map[string]interface{} {
	controllerValueMap := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": envConfig.ControllerImage.Repo,
			"tag":        envConfig.ControllerImage.Tag,
		},
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "controller",
		},
		"webhookMode": "disabled",
	}
	controllerValueMap["logLevel"] = logLevelValue(crdConfig)
	if envConfig.IsOpenshift {
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

func speakerValues(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB) map[string]interface{} {
	frrEnabled := false
	if envConfig.BGPType == params.FRRMode {
		frrEnabled = true
	}
	speakerValueMap := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": envConfig.SpeakerImage.Repo,
			"tag":        envConfig.SpeakerImage.Tag,
		},
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "speaker",
		},
		"frr": map[string]interface{}{
			"enabled": frrEnabled,
			"image": map[string]interface{}{
				"repository": envConfig.FRRImage.Repo,
				"tag":        envConfig.FRRImage.Tag,
			},
			"metricsPort":       envConfig.FRRMetricsPort,
			"secureMetricsPort": envConfig.SecureFRRMetricsPort,
		},
		"memberlist": map[string]interface{}{
			"enabled":    true,
			"mlBindPort": envConfig.MLBindPort,
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

func metalLBFrrk8sValues(envConfig params.EnvConfig) map[string]interface{} {
	enabled := false
	if envConfig.BGPType == params.FRRK8sMode {
		enabled = true
	}
	frrk8sValuesMap := map[string]interface{}{
		"enabled": enabled,
	}
	return frrk8sValuesMap
}
