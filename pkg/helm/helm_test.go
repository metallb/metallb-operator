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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var update = flag.Bool("update", false, "update .golden files")

const (
	invalidHelmChartPath = "../../bindata/deployment/no-helm"
	helmChartPath        = "../../bindata/deployment/helm"
	helmChartName        = "metallb"
	MetalLBTestNameSpace = "metallb-test-namespace"
	speakerDaemonSet     = "speaker"
)

type envVar struct {
	key   string
	value string
}

func TestLoadMetalLBChart(t *testing.T) {
	resetEnv()
	oldServiceMonitorAvailable := serviceMonitorAvailable
	serviceMonitorAvailable = func(_ client.Client) bool {
		return true
	}
	defer func() { serviceMonitorAvailable = oldServiceMonitorAvailable }()

	g := NewGomegaWithT(t)
	setEnv()
	_, err := InitMetalLBChart(invalidHelmChartPath, helmChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).NotTo(BeNil())
	chart, err := InitMetalLBChart(helmChartPath, helmChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).To(BeNil())
	g.Expect(chart.chart).NotTo(BeNil())
	g.Expect(chart.chart.Name()).To(Equal(helmChartName))
}

func TestParseMetalLBChartWithCustomValues(t *testing.T) {
	resetEnv()

	oldServiceMonitorAvailable := serviceMonitorAvailable
	serviceMonitorAvailable = func(_ client.Client) bool {
		return true
	}
	defer func() { serviceMonitorAvailable = oldServiceMonitorAvailable }()

	g := NewGomegaWithT(t)
	setEnv()
	chart, err := InitMetalLBChart(helmChartPath, helmChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).To(BeNil())
	speakerTolerations := []v1.Toleration{
		{
			Key:      "example1",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoExecute,
		},
	}
	speakerNodeSelector := map[string]string{"kubernetes.io/os": "linux", "node-role.kubernetes.io/worker": "true"}
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: metallbv1beta1.MetalLBSpec{
			SpeakerNodeSelector: speakerNodeSelector,
			SpeakerTolerations:  speakerTolerations,
		},
	}

	objs, err := chart.GetObjects(metallb)
	g.Expect(err).To(BeNil())
	var isSpeakerFound bool
	for _, obj := range objs {
		objKind := obj.GetKind()
		if objKind == "DaemonSet" {
			g.Expect(obj.GetName()).To(Equal(speakerDaemonSet))
			speaker := appsv1.DaemonSet{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &speaker)
			g.Expect(err).To(BeNil())
			g.Expect(speaker.GetName()).To(Equal(speakerDaemonSet))
			g.Expect(speaker.Spec.Template.Spec.Tolerations).To(Equal([]v1.Toleration{
				{
					Key:               "node-role.kubernetes.io/master",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoSchedule",
					TolerationSeconds: nil,
				},
				{
					Key:               "node-role.kubernetes.io/control-plane",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoSchedule",
					TolerationSeconds: nil,
				},
				{
					Key:      "example1",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoExecute,
				},
			}))
			g.Expect(speaker.Spec.Template.Spec.NodeSelector).To(Equal(speakerNodeSelector))
			isSpeakerFound = true
		}
	}
	g.Expect(isSpeakerFound).To(BeTrue())
}

func TestParseOCPSecureMetrics(t *testing.T) {
	oldServiceMonitorAvailable := serviceMonitorAvailable
	serviceMonitorAvailable = func(_ client.Client) bool {
		return true
	}
	defer func() { serviceMonitorAvailable = oldServiceMonitorAvailable }()
	resetEnv()
	setEnv(envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"HTTPS_METRICS_PORT", "9998"},
		envVar{"FRR_HTTPS_METRICS_PORT", "9999"},
		envVar{"METALLB_BGP_TYPE", "frr"},
	)
	g := NewGomegaWithT(t)
	setEnv()
	chart, err := InitMetalLBChart(helmChartPath, helmChartName, MetalLBTestNameSpace, nil, true)
	g.Expect(err).To(BeNil())
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
	}

	objs, err := chart.GetObjects(metallb)
	g.Expect(err).To(BeNil())
	for _, obj := range objs {
		objKind := obj.GetKind()
		if objKind == "DaemonSet" {
			err = validateObject("ocp", "speaker", obj)
			g.Expect(err).NotTo(HaveOccurred())
		}
	}
}

func validateObject(testcase, name string, obj *unstructured.Unstructured) error {
	goldenFile := filepath.Join("testdata", testcase+"-"+name+".golden")
	j, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	if *update {
		if err := ioutil.WriteFile(goldenFile, j, 0644); err != nil {
			return err
		}
	}

	expected, err := ioutil.ReadFile(goldenFile)
	if err != nil {
		return err
	}

	if !cmp.Equal(expected, j) {
		return fmt.Errorf("failed. (-want +got):\n%s", cmp.Diff(string(expected), string(j)))
	}
	return nil
}

func resetEnv() {
	os.Setenv("CONTROLLER_IMAGE", "quay.io/metallb/controller")
	os.Setenv("SPEAKER_IMAGE", "quay.io/metallb/speaker")
	os.Setenv("FRR_IMAGE", "frrouting/frr:v7.5.1")
	os.Setenv("KUBE_RBAC_PROXY_IMAGE", "gcr.io/kubebuilder/kube-rbac-proxy:v0.12.0")
	os.Setenv("DEPLOY_SERVICEMONITORS", "false")
	os.Setenv("METALLB_BGP_TYPE", "native")

	os.Setenv("HTTPS_METRICS_PORT", "0")
	os.Setenv("FRR_HTTPS_METRICS_PORT", "0")
}

func setEnv(envs ...envVar) {
	for _, e := range envs {
		os.Setenv(e.key, e.value)
	}
}
