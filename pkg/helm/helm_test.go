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
	"os"
	"testing"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	invalidHelmChartPath = "../../bindata/deployment/no-helm"
	helmChartPath        = "../../bindata/deployment/helm"
	helmChartName        = "metallb"
	MetalLBTestNameSpace = "metallb-test-namespace"
	speakerDaemonSet     = "speaker"
)

func TestLoadMetalLBChart(t *testing.T) {
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

func setEnv() {
	os.Setenv("CONTROLLER_IMAGE", "quay.io/metallb/controller")
	os.Setenv("SPEAKER_IMAGE", "quay.io/metallb/speaker")
	os.Setenv("FRR_IMAGE", "frrouting/frr:v7.5.1")
}
