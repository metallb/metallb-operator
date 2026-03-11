package params

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFromEnvironment(t *testing.T) {

	tests := []struct {
		desc        string
		setup       func()
		expected    EnvConfig
		expectedErr bool
	}{
		{
			desc: "basics",
			setup: func() {
				setBasics()
			},
			expected: EnvConfig{
				Namespace: "test-namespace",
				ControllerImage: ImageInfo{
					Repo: "test-controller-image",
					Tag:  "1",
				},
				SpeakerImage: ImageInfo{
					Repo: "test-speaker-image",
					Tag:  "2",
				},
				FRRImage: ImageInfo{
					Repo: "test-frr-image",
					Tag:  "3",
				},
				KubeRBacImage: ImageInfo{
					Repo: "test-kube-rbac-proxy-image",
					Tag:  "4",
				},
				FRRK8sImage: ImageInfo{
					Repo: "test-frrk8s-image",
					Tag:  "5",
				},
				MLBindPort:                 7946,
				MetricsPort:                7472,
				FRRMetricsPort:             7473,
				FRRK8sMetricsPort:          7572,
				FRRK8sFRRMetricsPort:       7573,
				SecureFRRK8sMetricsPort:    9140,
				SecureFRRK8sFRRMetricsPort: 9141,
			},
		},
		{
			desc: "override ports",
			setup: func() {
				setBasics()
				_ = os.Setenv("DEPLOY_SERVICEMONITORS", "true")

				_ = os.Setenv("MEMBER_LIST_BIND_PORT", "1111")
				_ = os.Setenv("FRR_METRICS_PORT", "2222")
				_ = os.Setenv("FRR_HTTPS_METRICS_PORT", "3333")
				_ = os.Setenv("METRICS_PORT", "4444")
				_ = os.Setenv("HTTPS_METRICS_PORT", "5555")
				_ = os.Setenv("FRRK8S_FRR_METRICS_PORT", "6666")
				_ = os.Setenv("FRRK8S_HTTPS_METRICS_PORT", "7777")
				_ = os.Setenv("FRRK8S_FRR_HTTPS_METRICS_PORT", "8888")
				_ = os.Setenv("FRRK8S_METRICS_PORT", "9999")
			},
			expected: EnvConfig{
				Namespace: "test-namespace",
				ControllerImage: ImageInfo{
					Repo: "test-controller-image",
					Tag:  "1",
				},
				SpeakerImage: ImageInfo{
					Repo: "test-speaker-image",
					Tag:  "2",
				},
				FRRImage: ImageInfo{
					Repo: "test-frr-image",
					Tag:  "3",
				},
				KubeRBacImage: ImageInfo{
					Repo: "test-kube-rbac-proxy-image",
					Tag:  "4",
				},
				FRRK8sImage: ImageInfo{
					Repo: "test-frrk8s-image",
					Tag:  "5",
				},
				MLBindPort:                 1111,
				MetricsPort:                4444,
				SecureMetricsPort:          5555,
				FRRMetricsPort:             2222,
				SecureFRRMetricsPort:       3333,
				FRRK8sMetricsPort:          9999,
				FRRK8sFRRMetricsPort:       6666,
				SecureFRRK8sMetricsPort:    7777,
				SecureFRRK8sFRRMetricsPort: 8888,
				DeployServiceMonitors:      true,
			},
		},
		{
			desc: "with network policies enabled",
			setup: func() {
				setBasics()
				_ = os.Setenv("DISABLE_NETWORK_POLICIES", "true")
			},
			expected: EnvConfig{
				Namespace: "test-namespace",
				ControllerImage: ImageInfo{
					Repo: "test-controller-image",
					Tag:  "1",
				},
				SpeakerImage: ImageInfo{
					Repo: "test-speaker-image",
					Tag:  "2",
				},
				FRRImage: ImageInfo{
					Repo: "test-frr-image",
					Tag:  "3",
				},
				KubeRBacImage: ImageInfo{
					Repo: "test-kube-rbac-proxy-image",
					Tag:  "4",
				},
				FRRK8sImage: ImageInfo{
					Repo: "test-frrk8s-image",
					Tag:  "5",
				},
				MLBindPort:                 7946,
				MetricsPort:                7472,
				FRRMetricsPort:             7473,
				FRRK8sMetricsPort:          7572,
				FRRK8sFRRMetricsPort:       7573,
				SecureFRRK8sMetricsPort:    9140,
				SecureFRRK8sFRRMetricsPort: 9141,
				DisableNetworkPolicies:     true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			unset()
			test.setup()
			res, err := FromEnvironment(false)
			if err != nil && !test.expectedErr {
				t.Errorf("Unexpected error: %v", err)
			}
			if err == nil && test.expectedErr {
				t.Errorf("Expected error, got nil")
			}

			if res != test.expected {
				t.Errorf("res different from expected, %s", cmp.Diff(res, test.expected))
			}
		})
	}
}

func unset() {
	_ = os.Unsetenv("OPERATOR_NAMESPACE")
	_ = os.Unsetenv("CONTROLLER_IMAGE")
	_ = os.Unsetenv("SPEAKER_IMAGE")
	_ = os.Unsetenv("FRR_IMAGE")
	_ = os.Unsetenv("KUBE_RBAC_PROXY_IMAGE")
	_ = os.Unsetenv("MEMBER_LIST_BIND_PORT")
	_ = os.Unsetenv("FRR_METRICS_PORT")
	_ = os.Unsetenv("FRR_HTTPS_METRICS_PORT")
	_ = os.Unsetenv("METRICS_PORT")
	_ = os.Unsetenv("HTTPS_METRICS_PORT")
	_ = os.Unsetenv("FRRK8S_FRR_METRICS_PORT")
	_ = os.Unsetenv("FRRK8S_HTTPS_METRICS_PORT")
	_ = os.Unsetenv("FRRK8S_FRR_HTTPS_METRICS_PORT")
	_ = os.Unsetenv("FRRK8S_METRICS_PORT")
	_ = os.Unsetenv("DEPLOY_PODMONITORS")
	_ = os.Unsetenv("DEPLOY_SERVICEMONITORS")
	_ = os.Unsetenv("DISABLE_NETWORK_POLICIES")
	_ = os.Unsetenv("KUBE_RBAC_PROXY_IMAGE")
}

func setBasics() {
	_ = os.Setenv("OPERATOR_NAMESPACE", "test-namespace")
	_ = os.Setenv("CONTROLLER_IMAGE", "test-controller-image:1")
	_ = os.Setenv("SPEAKER_IMAGE", "test-speaker-image:2")
	_ = os.Setenv("FRR_IMAGE", "test-frr-image:3")
	_ = os.Setenv("KUBE_RBAC_PROXY_IMAGE", "test-kube-rbac-proxy-image:4")
	_ = os.Setenv("FRRK8S_IMAGE", "test-frrk8s-image:5")
}
