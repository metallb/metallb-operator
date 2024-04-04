package helm

import (
	"fmt"
	"net"
	"slices"
	"strings"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/params"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	frrk8sWebhookDeploymentName = "frr-k8s-webhook-server"
	frrk8sWebhookServiceName    = "frr-k8s-webhook-service"
	frrk8sWebhookSecretName     = "frr-k8s-webhook-server-cert"
	frrk8sValidatingWebhookName = "frr-k8s-validating-webhook-configuration"
)

// FRRK8SChart contains references which helps to retrieve manifest
// from chart after patching given custom values.
type FRRK8SChart struct {
	client      *action.Install
	envSettings *cli.EnvSettings
	chart       *chart.Chart
}

// NewFRRK8SChart initializes frr-k8s helm chart after loading it from given
// chart path and creating config object from environment variables.
func NewFRRK8SChart(path, name, namespace string) (*FRRK8SChart, error) {
	chart := &FRRK8SChart{}
	chart.envSettings = cli.New()
	chart.client = action.NewInstall(new(action.Configuration))
	chart.client.ReleaseName = name
	chart.client.DryRun = true
	chart.client.ClientOnly = true
	chart.client.Namespace = namespace
	chartPath, err := chart.client.ChartPathOptions.LocateChart(path, chart.envSettings)
	if err != nil {
		return nil, err
	}
	chart.chart, err = loader.Load(chartPath)
	if err != nil {
		return nil, err
	}
	return chart, nil
}

// Objects retrieves manifests from chart after patching custom values passed in crdConfig
// and environment variables.
func (h *FRRK8SChart) Objects(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB) ([]*unstructured.Unstructured, error) {
	chartValueOpts := &values.Options{}
	chartValues, err := chartValueOpts.MergeValues(getter.All(h.envSettings))
	if err != nil {
		return nil, err
	}

	err = patchChartValues(envConfig, crdConfig, chartValues)
	if err != nil {
		return nil, err
	}

	release, err := h.client.Run(h.chart, chartValues)
	if err != nil {
		return nil, err
	}
	objs, err := parseManifest(release.Manifest)
	if err != nil {
		return nil, err
	}
	res := []*unstructured.Unstructured{}
	for _, obj := range objs {
		// Set namespace explicitly into non cluster-scoped resource because helm doesn't
		// patch namespace into manifests at client.Run.
		objKind := obj.GetKind()
		if objKind != "PodSecurityPolicy" {
			obj.SetNamespace(envConfig.Namespace)
		}

		if isFRRK8SWebhookSecret(obj) && envConfig.IsOpenshift {
			// We want to skip creating the secret on OpenShift since it is created and managed
			// via the serving-cert-secret-name annotation on the service.
			continue
		}

		if isFRRK8SValidatingWebhook(obj) && envConfig.IsOpenshift {
			err := updateAnnotations(obj, map[string]string{"service.beta.openshift.io/inject-cabundle": "true"})
			if err != nil {
				return nil, err
			}
		}

		if isFRRK8SWebhookService(obj) && envConfig.IsOpenshift {
			err := updateAnnotations(obj, map[string]string{"service.beta.openshift.io/serving-cert-secret-name": frrk8sWebhookSecretName})
			if err != nil {
				return nil, err
			}
		}

		// we need to override the security context as helm values are added on top
		// of hardcoded ones in values.yaml, so it's not possible to reset runAsUser
		if isFRRK8SWebhookDeployment(obj) && envConfig.IsOpenshift {
			securityContext := map[string]interface{}{
				"runAsNonRoot": true,
			}
			err := unstructured.SetNestedMap(obj.Object, securityContext, "spec", "template", "spec", "securityContext")
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

		res = append(res, obj)
	}
	return res, nil
}

func patchChartValues(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB, valuesMap map[string]interface{}) error {
	var err error
	valuesMap["frrk8s"], err = frrk8sValues(envConfig, crdConfig)
	if err != nil {
		return err
	}
	valuesMap["prometheus"] = prometheusValues(envConfig)
	return nil
}

func frrk8sValues(envConfig params.EnvConfig, crdConfig *metallbv1beta1.MetalLB) (map[string]interface{}, error) {
	frrk8sValueMap := map[string]interface{}{
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "frr-k8s-daemon",
		},
		"frr": map[string]interface{}{
			"image": map[string]interface{}{
				"repository": envConfig.FRRImage.Repo,
				"tag":        envConfig.FRRImage.Tag,
			},
			"metricsPort":       envConfig.FRRK8sFRRMetricsPort,
			"secureMetricsPort": envConfig.SecureFRRK8sFRRMetricsPort,
		},
	}
	if envConfig.FRRK8sImage.Repo != "" {
		frrk8sValueMap["image"] = map[string]interface{}{
			"repository": envConfig.FRRK8sImage.Repo,
			"tag":        envConfig.FRRK8sImage.Tag,
		}
	}
	frrk8sValueMap["logLevel"] = logLevelValue(crdConfig)
	frrk8sValueMap["restartOnRotatorSecretRefresh"] = true

	if envConfig.IsOpenshift {
		// OpenShift is responsible of managing the cert secret
		frrk8sValueMap["disableCertRotation"] = true
		frrk8sValueMap["restartOnRotatorSecretRefresh"] = nil // the cert rotator isn't started anyways
	}

	// Mirror the behaviour of the speaker pods as frrk8s pods must follow the
	// speaker pods.
	if crdConfig.Spec.SpeakerNodeSelector != nil {
		frrk8sValueMap["nodeSelector"] = toInterfaceMap(crdConfig.Spec.SpeakerNodeSelector)
	}
	if crdConfig.Spec.SpeakerTolerations != nil {
		frrk8sValueMap["tolerations"] = crdConfig.Spec.SpeakerTolerations
	}

	if crdConfig.Spec.FRRK8SConfig != nil {
		var err error
		frrk8sValueMap["alwaysBlock"], err = alwaysBlockToString(crdConfig.Spec.FRRK8SConfig.AlwaysBlock)
		if err != nil {
			return nil, err
		}
	}
	return frrk8sValueMap, nil
}

func prometheusValues(envConfig params.EnvConfig) map[string]interface{} {
	tlsConfig := map[string]interface{}{
		"insecureSkipVerify": true,
	}
	annotations := map[string]interface{}{}
	tlsSecret := ""

	if envConfig.IsOpenshift {
		tlsConfig, annotations, tlsSecret = ocpPromConfigFor("frr-k8s", envConfig.Namespace)
	}

	serviceMonitor := map[string]interface{}{
		"enabled":     false,
		"annotations": annotations,
		"tlsConfig":   tlsConfig,
	}

	if envConfig.DeployServiceMonitors {
		serviceMonitor["enabled"] = true
		serviceMonitor["metricRelabelings"] = []map[string]interface{}{
			{
				"regex":        "frrk8s_bgp_(.*)",
				"replacement":  "metallb_bgp_$1",
				"sourceLabels": []string{"__name__"},
				"targetLabel":  "__name__",
			},
			{
				"regex":        "frrk8s_bfd_(.*)",
				"replacement":  "metallb_bfd_$1",
				"sourceLabels": []string{"__name__"},
				"targetLabel":  "__name__",
			},
		}
	}

	return map[string]interface{}{
		"metricsPort":       envConfig.FRRK8sMetricsPort,
		"secureMetricsPort": envConfig.SecureFRRK8sMetricsPort,
		"serviceMonitor":    serviceMonitor,
		"rbacProxy": map[string]interface{}{
			"repository": envConfig.KubeRBacImage.Repo,
			"tag":        envConfig.KubeRBacImage.Tag,
		},
		"serviceAccount":   "foo", // required by the chart, we won't render roles or rolebindings anyway
		"namespace":        "bar",
		"metricsTLSSecret": tlsSecret,
	}
}

func isFRRK8SWebhookDeployment(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Deployment" && obj.GetName() == frrk8sWebhookDeploymentName
}

func isFRRK8SWebhookService(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Service" && obj.GetName() == frrk8sWebhookServiceName
}

func isFRRK8SWebhookSecret(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Secret" && obj.GetName() == frrk8sWebhookSecretName
}

func isFRRK8SValidatingWebhook(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "ValidatingWebhookConfiguration" && obj.GetName() == frrk8sValidatingWebhookName
}

func alwaysBlockToString(alwaysBlock []string) (string, error) {
	toSort := make([]string, len(alwaysBlock))
	copy(toSort, alwaysBlock)
	for _, cidr := range alwaysBlock {
		_, _, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return "", fmt.Errorf("invalid CIDR %s in AlwaysBlock", cidr)
		}
	}

	slices.Sort(toSort)
	return strings.Join(toSort, ","), nil
}
