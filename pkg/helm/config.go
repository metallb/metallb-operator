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
	"strconv"
	"strings"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/pkg/errors"
)

type chartConfig struct {
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

type imageInfo struct {
	repo string
	tag  string
}

func patchToChartValues(c *chartConfig, crdConfig *metallbv1beta1.MetalLB, withPrometheus bool, valueMap map[string]interface{}) {
	withPrometheusValues(c, valueMap)
	withControllerValues(c, crdConfig, valueMap)
	withSpeakerValues(c, crdConfig, valueMap)
}

func withPrometheusValues(c *chartConfig, valueMap map[string]interface{}) {
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
		speakerTLSConfig = map[string]interface{}{
			"caFile":             "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt",
			"serverName":         fmt.Sprintf("speaker-monitor-service.%s.svc", c.namespace),
			"certFile":           "/etc/prometheus/secrets/metrics-client-certs/tls.crt",
			"keyFile":            "/etc/prometheus/secrets/metrics-client-certs/tls.key",
			"insecureSkipVerify": false,
		}
		controllerTLSConfig = map[string]interface{}{
			"caFile":             "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt",
			"serverName":         fmt.Sprintf("controller-monitor-service.%s.svc", c.namespace),
			"certFile":           "/etc/prometheus/secrets/metrics-client-certs/tls.crt",
			"keyFile":            "/etc/prometheus/secrets/metrics-client-certs/tls.key",
			"insecureSkipVerify": false,
		}
		speakerAnnotations = map[string]interface{}{
			"service.beta.openshift.io/serving-cert-secret-name": "speaker-certs-secret",
		}
		controllerAnnotations = map[string]interface{}{
			"service.beta.openshift.io/serving-cert-secret-name": "controller-certs-secret",
		}
		speakerTLSSecret = "speaker-certs-secret"
		controllerTLSSecret = "controller-certs-secret"
	}

	valueMap["prometheus"] = map[string]interface{}{
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

func withControllerValues(c *chartConfig, crdConfig *metallbv1beta1.MetalLB, valueMap map[string]interface{}) {
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
	withCommonValues(crdConfig, controllerValueMap)
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
	valueMap["controller"] = controllerValueMap
}

func withSpeakerValues(c *chartConfig, crdConfig *metallbv1beta1.MetalLB, valueMap map[string]interface{}) {
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
	withCommonValues(crdConfig, speakerValueMap)
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
	valueMap["speaker"] = speakerValueMap
}

func withCommonValues(crdConfig *metallbv1beta1.MetalLB, manifestValueMap map[string]interface{}) {
	logLevel := metallbv1beta1.LogLevelInfo
	if crdConfig.Spec.LogLevel != "" {
		logLevel = crdConfig.Spec.LogLevel
	}
	manifestValueMap["logLevel"] = logLevel
}

func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

func loadConfig(namespace string, isOCP bool) (*chartConfig, error) {
	config := &chartConfig{
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
	if os.Getenv("METALLB_BGP_TYPE") == bgpFrr {
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
	err = validateConfig(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func validateConfig(c *chartConfig) error {
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

func getImageNameTag(envValue string) (string, string) {
	img := strings.Split(envValue, ":")
	if len(img) == 1 {
		return img[0], ""
	}
	return img[0], img[1]
}
