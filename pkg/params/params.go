package params

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/metallb/metallb-operator/api/v1beta1"
)

type ImageInfo struct {
	Repo string
	Tag  string
}

func BGPType(m *v1beta1.MetalLB, env EnvConfig) v1beta1.BGPType {
	if m.Spec.BGPBackend != "" {
		return m.Spec.BGPBackend
	}
	if env.IsOpenshift && env.MustDeployFRRK8sFromCNO {
		return v1beta1.FRRK8sExternalMode
	}
	if env.IsOpenshift {
		return v1beta1.FRRK8sMode
	}
	return v1beta1.FRRMode
}

type EnvConfig struct {
	Namespace                  string
	FRRK8sExternalNamespace    string
	ControllerImage            ImageInfo
	SpeakerImage               ImageInfo
	FRRImage                   ImageInfo
	FRRK8sImage                ImageInfo
	KubeRBacImage              ImageInfo
	CNOMinFRRK8sVersion        string
	MLBindPort                 int
	FRRMetricsPort             int
	SecureFRRMetricsPort       int
	FRRK8sMetricsPort          int
	FRRK8sFRRMetricsPort       int
	SecureFRRK8sMetricsPort    int
	SecureFRRK8sFRRMetricsPort int
	MetricsPort                int
	SecureMetricsPort          int
	DeployPodMonitors          bool
	DeployServiceMonitors      bool
	DisableNetworkPolicies     bool
	IsOpenshift                bool
	MustDeployFRRK8sFromCNO    bool
}

func FromEnvironment(isOpenshift bool) (EnvConfig, error) {
	res := EnvConfig{}
	found := false
	res.IsOpenshift = isOpenshift
	res.Namespace, found = os.LookupEnv("OPERATOR_NAMESPACE")
	if !found {
		return EnvConfig{}, errors.New("missing mandatory OPERATOR_NAMESPACE env variable")
	}
	var err error
	res.ControllerImage, err = imageFromEnv("CONTROLLER_IMAGE")
	if err != nil {
		return EnvConfig{}, err
	}
	res.SpeakerImage, err = imageFromEnv("SPEAKER_IMAGE")
	if err != nil {
		return EnvConfig{}, err
	}

	// FRR Image is mandatory only in frr mode
	res.FRRImage, err = imageFromEnv("FRR_IMAGE")
	if err != nil {
		return EnvConfig{}, fmt.Errorf("FRRImage is mandatory for frr mode, %w", err)
	}

	res.KubeRBacImage, err = imageFromEnv("KUBE_RBAC_PROXY_IMAGE")
	if err != nil {
		return EnvConfig{}, err
	}

	res.MLBindPort, err = intValueWithDefault("MEMBER_LIST_BIND_PORT", 7946)
	if err != nil {
		return EnvConfig{}, err
	}
	res.FRRMetricsPort, err = intValueWithDefault("FRR_METRICS_PORT", 7473)
	if err != nil {
		return EnvConfig{}, err
	}
	res.SecureFRRMetricsPort, err = intValueWithDefault("FRR_HTTPS_METRICS_PORT", 0)
	if err != nil {
		return EnvConfig{}, err
	}
	res.MetricsPort, err = intValueWithDefault("METRICS_PORT", 7472)
	if err != nil {
		return EnvConfig{}, err
	}
	res.SecureMetricsPort, err = intValueWithDefault("HTTPS_METRICS_PORT", 0)
	if err != nil {
		return EnvConfig{}, err
	}
	res.FRRK8sMetricsPort, err = intValueWithDefault("FRRK8S_METRICS_PORT", 7572)
	if err != nil {
		return EnvConfig{}, err
	}

	res.SecureFRRK8sMetricsPort, err = intValueWithDefault("FRRK8S_HTTPS_METRICS_PORT", 9140)
	if err != nil {
		return EnvConfig{}, err
	}

	res.FRRK8sFRRMetricsPort, err = intValueWithDefault("FRRK8S_FRR_METRICS_PORT", 7573)
	if err != nil {
		return EnvConfig{}, err
	}
	res.SecureFRRK8sFRRMetricsPort, err = intValueWithDefault("FRRK8S_FRR_HTTPS_METRICS_PORT", 9141)
	if err != nil {
		return EnvConfig{}, err
	}

	if os.Getenv("DEPLOY_PODMONITORS") == "true" {
		res.DeployPodMonitors = true
	}
	if os.Getenv("DEPLOY_SERVICEMONITORS") == "true" {
		res.DeployServiceMonitors = true
	}
	if os.Getenv("DISABLE_NETWORK_POLICIES") == "true" {
		res.DisableNetworkPolicies = true
	}

	res.FRRK8sExternalNamespace = os.Getenv("FRRK8S_EXTERNAL_NAMESPACE")
	if os.Getenv("DEPLOY_FRRK8S_FROM_CNO") == "true" {
		res.MustDeployFRRK8sFromCNO = true
	}

	// Ignoring the error, if not set we'll consume the image from the chart
	res.FRRK8sImage, _ = imageFromEnv("FRRK8S_IMAGE")
	res.CNOMinFRRK8sVersion = os.Getenv("CNO_MIN_FRRK8S_VERSION")

	err = validate(res)
	if err != nil {
		return EnvConfig{}, err
	}

	return res, nil
}

func validate(config EnvConfig) error {
	if config.DeployPodMonitors && config.DeployServiceMonitors {
		return fmt.Errorf("pod monitors and service monitors are mutually exclusive, only one can be enabled")
	}
	if config.SecureMetricsPort != 0 && !config.DeployServiceMonitors {
		return fmt.Errorf("secureMetricsPort is available only if service monitors are enabled")
	}
	if config.SecureFRRMetricsPort != 0 && !config.DeployServiceMonitors {
		return fmt.Errorf("secureFRRMetricsPort is available only if service monitors are enabled")
	}
	if config.CNOMinFRRK8sVersion != "" {
		_, err := semver.NewVersion(config.CNOMinFRRK8sVersion)
		if err != nil {
			return fmt.Errorf("invalid cno min frrk8s supported version: %w", err)
		}
	}
	return nil
}

func imageFromEnv(imageEnv string) (ImageInfo, error) {
	res := ImageInfo{}
	value, found := os.LookupEnv(imageEnv)
	if !found {
		return res, fmt.Errorf("%s environment value not set", imageEnv)
	}
	res.Repo, res.Tag = getImageNameTag(value)
	return res, nil
}

func getImageNameTag(envValue string) (string, string) {
	pos := strings.LastIndex(envValue, "/")
	// We assume the last ":" shows up right before the image's tag, and the last "/" just before the image's name.
	// Multiple ":" can be present when the port of the registry is specified and we should include
	// it as part of the repo's url.
	img := strings.Split(envValue[pos+1:], ":")
	repoPath := envValue[:pos+1]

	if len(img) == 1 {
		return repoPath + img[0], ""
	}
	return repoPath + img[0], img[1]
}

func intValueWithDefault(name string, def int) (int, error) {
	val := os.Getenv(name)
	if val != "" {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("Failed to convert %s from %s to int: %w", val, name, err)
		}
		return res, nil
	}
	return def, nil
}
