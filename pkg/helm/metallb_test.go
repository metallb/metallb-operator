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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var update = flag.Bool("update", false, "update .golden files")

const (
	invalidMetalLBChartPath = "../../bindata/deployment/no-helm"
	metalLBChartPath        = "../../bindata/deployment/helm/metallb"
	metalLBChartName        = "metallb"
	MetalLBTestNameSpace    = "metallb-test-namespace"
	speakerDaemonSet        = "speaker"
	controllerDeployment    = "controller"
)

type envVar struct {
	key   string
	value string
}

func TestLoadMetalLBChart(t *testing.T) {
	resetEnv()
	g := NewGomegaWithT(t)
	setEnv()
	_, err := NewMetalLBChart(invalidMetalLBChartPath, metalLBChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).NotTo(BeNil())
	chart, err := NewMetalLBChart(metalLBChartPath, metalLBChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).To(BeNil())
	g.Expect(chart.chart).NotTo(BeNil())
	g.Expect(chart.chart.Name()).To(Equal(metalLBChartName))
}

func TestParseMetalLBChartWithCustomValues(t *testing.T) {
	resetEnv()

	g := NewGomegaWithT(t)
	setEnv()
	chart, err := NewMetalLBChart(metalLBChartPath, metalLBChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).To(BeNil())
	speakerTolerations := []v1.Toleration{
		{
			Key:      "example1",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoExecute,
		},
	}
	speakerNodeSelector := map[string]string{"kubernetes.io/os": "linux", "node-role.kubernetes.io/worker": "true"}
	controllerTolerations := []v1.Toleration{
		{
			Key:      "example2",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoExecute,
		},
	}
	loadBalancerClass := "metallb.universe.tf/metallb"
	controllerNodeSelector := map[string]string{"kubernetes.io/os": "linux", "node-role.kubernetes.io/worker": "true"}
	controllerConfig := &metallbv1beta1.Config{
		PriorityClassName: "high-priority",
		RuntimeClassName:  "cri-o",
		Annotations:       map[string]string{"unittest": "controller"},
		Resources:         &v1.ResourceRequirements{Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI)}},
		Affinity: &v1.Affinity{PodAffinity: &v1.PodAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "metallb",
			}}}}}},
	}
	speakerConfig := &metallbv1beta1.Config{
		PriorityClassName: "high-priority",
		RuntimeClassName:  "cri-o",
		Annotations:       map[string]string{"unittest": "speaker"},
		Resources:         &v1.ResourceRequirements{Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: *resource.NewMilliQuantity(200, resource.DecimalSI)}},
		Affinity: &v1.Affinity{PodAffinity: &v1.PodAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "metallb",
			}}}}}},
	}
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: metallbv1beta1.MetalLBSpec{
			SpeakerNodeSelector:    speakerNodeSelector,
			SpeakerTolerations:     speakerTolerations,
			ControllerNodeSelector: controllerNodeSelector,
			ControllerTolerations:  controllerTolerations,
			ControllerConfig:       controllerConfig,
			SpeakerConfig:          speakerConfig,
			LoadBalancerClass:      loadBalancerClass,
		},
	}

	objs, err := chart.Objects(metallb, false)
	g.Expect(err).To(BeNil())
	var isSpeakerFound, isControllerFound bool
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
			g.Expect(speaker.Spec.Template.Spec.PriorityClassName).To(Equal("high-priority"))
			g.Expect(*speaker.Spec.Template.Spec.RuntimeClassName).To(Equal("cri-o"))
			g.Expect(speaker.Spec.Template.Annotations["unittest"]).To(Equal("speaker"))
			g.Expect(speaker.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["app"]).To(Equal("metallb"))
			var speakerContainerFound bool
			for _, container := range speaker.Spec.Template.Spec.Containers {
				if container.Name == "speaker" {
					g.Expect(container.Resources).NotTo(BeNil())
					g.Expect(container.Resources.Limits.Cpu().MilliValue()).To(Equal(int64(200)))
					g.Expect(container.Args).To(ContainElement("--lb-class=metallb.universe.tf/metallb"))
					speakerContainerFound = true
				}
			}
			g.Expect(speakerContainerFound).To(BeTrue())
			isSpeakerFound = true
		}
		if objKind == "Deployment" {
			g.Expect(obj.GetName()).To(Equal(controllerDeployment))
			controller := appsv1.Deployment{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &controller)
			g.Expect(err).To(BeNil())
			g.Expect(controller.GetName()).To(Equal(controllerDeployment))
			g.Expect(controller.Spec.Template.Spec.Tolerations).To(Equal([]v1.Toleration{
				{
					Key:      "example2",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoExecute,
				},
			}))
			g.Expect(controller.Spec.Template.Spec.NodeSelector).To(Equal(controllerNodeSelector))
			g.Expect(controller.Spec.Template.Spec.PriorityClassName).To(Equal("high-priority"))
			g.Expect(*controller.Spec.Template.Spec.RuntimeClassName).To(Equal("cri-o"))
			g.Expect(controller.Spec.Template.Annotations["unittest"]).To(Equal("controller"))
			g.Expect(controller.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["app"]).To(Equal("metallb"))
			var controllerContainerFound bool
			for _, container := range controller.Spec.Template.Spec.Containers {
				if container.Name == "controller" {
					g.Expect(container.Resources).NotTo(BeNil())
					g.Expect(container.Resources.Limits.Cpu().MilliValue()).To(Equal(int64(100)))
					controllerContainerFound = true
				}
			}
			g.Expect(controllerContainerFound).To(BeTrue())
			isControllerFound = true
		}
	}
	g.Expect(isSpeakerFound).To(BeTrue())
	g.Expect(isControllerFound).To(BeTrue())
}

func TestParseOCPSecureMetrics(t *testing.T) {
	resetEnv()
	setEnv(envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"HTTPS_METRICS_PORT", "9998"},
		envVar{"FRR_HTTPS_METRICS_PORT", "9999"},
		envVar{"METALLB_BGP_TYPE", "frr"},
	)
	g := NewGomegaWithT(t)
	setEnv()
	chart, err := NewMetalLBChart(metalLBChartPath, metalLBChartName, MetalLBTestNameSpace, nil, true)
	g.Expect(err).To(BeNil())
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
	}

	objs, err := chart.Objects(metallb, true)
	g.Expect(err).To(BeNil())
	for _, obj := range objs {
		objKind := obj.GetKind()
		if objKind == "DaemonSet" {
			err = validateObject("ocp-metrics", "speaker", obj)
			if err != nil {
				t.Fatalf("test ocp-metrics-speaker failed. %s", err)
			}
		}
		if objKind == "ServiceMonitor" {
			err = validateObject("ocp-metrics", obj.GetName(), obj)
			if err != nil {
				t.Fatalf("test ocp-metrics-%s failed. %s", obj.GetName(), err)
			}
		}
		if objKind == "Deployment" {
			err = validateObject("ocp-metrics", "controller", obj)
			if err != nil {
				t.Fatalf("test ocp-metrics-controller failed. %s", err)
			}
		}
	}
}

func TestParseSecureMetrics(t *testing.T) {
	resetEnv()
	setEnv(envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"DEPLOY_SERVICEMONITORS", "true"},
		envVar{"HTTPS_METRICS_PORT", "9998"},
		envVar{"FRR_HTTPS_METRICS_PORT", "9999"},
		envVar{"METALLB_BGP_TYPE", "frr"},
		envVar{"KUBE_RBAC_PROXY_IMAGE", "myrepo/image:mytag"},
	)
	g := NewGomegaWithT(t)
	setEnv()
	chart, err := NewMetalLBChart(metalLBChartPath, metalLBChartName, MetalLBTestNameSpace, nil, false)
	g.Expect(err).To(BeNil())
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
	}

	objs, err := chart.Objects(metallb, true)
	g.Expect(err).To(BeNil())
	for _, obj := range objs {
		objKind := obj.GetKind()
		if objKind == "DaemonSet" {
			err = validateObject("vanilla-metrics", "speaker", obj)
			if err != nil {
				t.Fatalf("test vanilla-metrics-speaker failed. %s", err)
			}
		}
		if objKind == "ServiceMonitor" {
			err = validateObject("vanilla-metrics", obj.GetName(), obj)
			if err != nil {
				t.Fatalf("test vanilla-metrics-%s failed. %s", obj.GetName(), err)
			}
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
		if err := os.WriteFile(goldenFile, j, 0644); err != nil {
			return err
		}
	}

	expected, err := os.ReadFile(goldenFile)
	if err != nil {
		return err
	}

	if !cmp.Equal(expected, j) {
		return fmt.Errorf("unexpected manifest (-want +got):\n%s", cmp.Diff(string(expected), string(j)))
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

	os.Setenv("FRRK8S_IMAGE", "")
}

func setEnv(envs ...envVar) {
	for _, e := range envs {
		os.Setenv(e.key, e.value)
	}
}
