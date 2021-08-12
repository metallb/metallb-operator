package apply

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// TestReconcileNamespace makes sure that namespace
// annotations are merged, and everything else is overwritten
// Namespaces use the "generic" logic; deployments and services
// have custom logic
func TestMergeNamespace(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
  labels:
    a: cur
    b: cur
  annotations:
    a: cur
    b: cur`)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: Namespace
metadata:
  name: ns1
  labels:
    a: upd
    c: upd
  annotations:
    a: upd
    c: upd`)

	// this mutates updated
	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(upd.GetLabels()).To(Equal(map[string]string{
		"a": "upd",
		"b": "cur",
		"c": "upd",
	}))

	g.Expect(upd.GetAnnotations()).To(Equal(map[string]string{
		"a": "upd",
		"b": "cur",
		"c": "upd",
	}))
}

func TestMergeDeployment(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1
  labels:
    a: cur
    b: cur
  annotations:
    deployment.kubernetes.io/revision: cur
    a: cur
    b: cur`)

	upd := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1
  labels:
    a: upd
    c: upd
  annotations:
    deployment.kubernetes.io/revision: upd
    a: upd
    c: upd`)

	// this mutates updated
	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	// labels are not merged
	g.Expect(upd.GetLabels()).To(Equal(map[string]string{
		"a": "upd",
		"b": "cur",
		"c": "upd",
	}))

	// annotations are merged
	g.Expect(upd.GetAnnotations()).To(Equal(map[string]string{
		"a": "upd",
		"b": "cur",
		"c": "upd",

		"deployment.kubernetes.io/revision": "cur",
	}))
}

func TestMergeNilCur(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1`)

	upd := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1
  labels:
    a: upd
    c: upd
  annotations:
    a: upd
    c: upd`)

	// this mutates updated
	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(upd.GetLabels()).To(Equal(map[string]string{
		"a": "upd",
		"c": "upd",
	}))

	g.Expect(upd.GetAnnotations()).To(Equal(map[string]string{
		"a": "upd",
		"c": "upd",
	}))
}

func TestMergeNilMeta(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1`)

	upd := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1`)

	// this mutates updated
	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(upd.GetLabels()).To(BeEmpty())
}

func TestMergeNilUpd(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1
  labels:
    a: cur
    b: cur
  annotations:
    a: cur
    b: cur`)

	upd := UnstructuredFromYaml(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1`)

	// this mutates updated
	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(upd.GetLabels()).To(Equal(map[string]string{
		"a": "cur",
		"b": "cur",
	}))

	g.Expect(upd.GetAnnotations()).To(Equal(map[string]string{
		"a": "cur",
		"b": "cur",
	}))
}

func TestMergeService(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: Service
metadata:
  name: d1
spec:
  clusterIP: cur
  ipFamilies: ["IPv4"]
  ipFamilyPolicy: SingleStack`)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: Service
metadata:
  name: d1
spec:
  clusterIP: upd`)

	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	ip, _, err := uns.NestedString(upd.Object, "spec", "clusterIP")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ip).To(Equal("cur"))

	ipFamily, _, err := uns.NestedStringSlice(upd.Object, "spec", "ipFamilies")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ipFamily).To(Equal([]string{"IPv4"}))

	ipfp, _, err := uns.NestedString(upd.Object, "spec", "ipFamilyPolicy")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ipfp).To(Equal("SingleStack"))

	upd = UnstructuredFromYaml(t, `
apiVersion: v1
kind: Service
metadata:
  name: d1
spec:
  clusterIP: upd
  ipFamilyPolicy: RequireDualStack`)

	err = MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	ipfp, _, err = uns.NestedString(upd.Object, "spec", "ipFamilyPolicy")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ipfp).To(Equal("RequireDualStack"))

}

func TestMergeServiceAccount(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d1
  annotations:
    a: cur
secrets:
- foo`)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d1
  annotations:
    b: upd`)

	err := IsObjectSupported(cur)
	g.Expect(err).To(MatchError(ContainSubstring("cannot create ServiceAccount with secrets")))

	err = MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())

	s, ok, err := uns.NestedSlice(upd.Object, "secrets")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ok).To(BeTrue())
	g.Expect(s).To(ConsistOf("foo"))
}

// UnstructuredFromYaml creates an unstructured object from a raw yaml string
func UnstructuredFromYaml(t *testing.T, obj string) *uns.Unstructured {
	t.Helper()
	buf := bytes.NewBufferString(obj)
	decoder := yaml.NewYAMLOrJSONDecoder(buf, 4096)

	u := uns.Unstructured{}
	err := decoder.Decode(&u)
	if err != nil {
		t.Fatalf("failed to parse test yaml: %v", err)
	}

	return &u
}

func TestMergeAddressPoolSingleObject(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: gold
      protocol: layer2
      addresses:
      - 172.20.0.100/24
      auto-assign: false`)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: silver
      protocol: layer2
      addresses:
      - 172.22.0.100/24
      auto-assign: false`)

	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())
	configmap, _, err := uns.NestedStringMap(upd.Object, "data")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configmap[MetalLBConfigMap]).Should(MatchYAML(`address-pools:
- name: gold
  protocol: layer2
  addresses:
  - 172.20.0.100/24
  auto-assign: false
- name: silver
  protocol: layer2
  addresses:
  - 172.22.0.100/24
  auto-assign: false
`))
}

func TestMergeAddressPoolMultipleObjects(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: green
      protocol: layer2
      addresses:
      - 172.10.0.100/24
    - name: yellow
      protocol: layer2
      addresses:
      - 172.30.0.100/16
    - name: blue
      protocol: layer2
      addresses:
      - 172.20.0.100/24
      auto-assign: false`)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: yellow
      protocol: layer2
      addresses:
      - 172.30.0.100/24
    - name: blue
      protocol: layer2
      addresses:
      - 172.20.0.100/28
      auto-assign: false`)

	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())
	configmap, _, err := uns.NestedStringMap(upd.Object, "data")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configmap[MetalLBConfigMap]).Should(MatchYAML(`address-pools:
- name: green
  protocol: layer2
  addresses:
  - 172.10.0.100/24
- name: yellow
  protocol: layer2
  addresses:
  - 172.30.0.100/24
- name: blue
  protocol: layer2
  addresses:
  - 172.20.0.100/28
  auto-assign: false
`))
}

func TestMergeBGPPeerSingleObject(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    peers:
    - peer-address: 10.0.0.1
      peer-asn: 64501
      my-asn: 64500
      router-id: 10.10.10.10
      source-address: 11.0.0.1
      peer-port: 1
      hold-time: 10ns `)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    peers:
    - peer-address: 10.0.0.2
      peer-asn: 64502
      my-asn: 64502
      router-id: 20.20.20.20
      source-address: 12.0.0.1
      peer-port: 2
      hold-time: 20ns `)

	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())
	configmap, _, err := uns.NestedStringMap(upd.Object, "data")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configmap[MetalLBConfigMap]).Should(MatchYAML(`peers:
- peer-address: 10.0.0.1
  peer-asn: 64501
  my-asn: 64500
  router-id: 10.10.10.10
  source-address: 11.0.0.1
  peer-port: 1
  hold-time: 10ns
- peer-address: 10.0.0.2
  peer-asn: 64502
  my-asn: 64502
  router-id: 20.20.20.20
  source-address: 12.0.0.1
  peer-port: 2
  hold-time: 20ns
`))
}

func TestMergeAddressPoolAndBGPPeerWithBGPAdvObject(t *testing.T) {
	g := NewGomegaWithT(t)

	cur := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: gold
      protocol: bgp
      addresses:
      - 172.20.0.100/24
      auto-assign: false
      bgp-advertisements:
      - communities:
        - 65535:65282
        aggregation-length: 32
        localpref: 100
      - 
        communities:
        - 8000:800
        aggregation-length: 24
    peers:
    - peer-address: 20.0.0.1
      peer-asn: 64000
      my-asn: 65000
      router-id: 10.10.10.10
      source-address: 11.0.0.1
      peer-port: 1
      hold-time: 10ns `)

	upd := UnstructuredFromYaml(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  namespace: metallb-system
data:
  config: |
    address-pools:
    - name: silver
      protocol: bgp
      addresses:
      - 172.22.0.100/24
      auto-assign: false
      bgp-advertisements:
      - communities:
        - 7007:007
        - 7018:007
        aggregation-length: 16
        localpref: 200
    peers:
    - peer-address: 20.0.0.2
      peer-asn: 64001
      my-asn: 65001
      router-id: 20.20.20.20
      source-address: 12.0.0.1
      peer-port: 2
      hold-time: 20ns `)

	err := MergeObjectForUpdate(cur, upd)
	g.Expect(err).NotTo(HaveOccurred())
	configmap, _, err := uns.NestedStringMap(upd.Object, "data")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configmap[MetalLBConfigMap]).Should(MatchYAML(`address-pools:
- name: gold
  protocol: bgp
  addresses:
  - 172.20.0.100/24
  auto-assign: false
  bgp-advertisements:
  - communities:
    - 65535:65282
    aggregation-length: 32
    localpref: 100
  - communities:
    - 8000:800
    aggregation-length: 24
- name: silver
  protocol: bgp
  addresses:
  - 172.22.0.100/24
  auto-assign: false
  bgp-advertisements:
  - communities:
    - 7007:007
    - 7018:007
    aggregation-length: 16
    localpref: 200
peers:
- peer-address: 20.0.0.1
  peer-asn: 64000
  my-asn: 65000
  router-id: 10.10.10.10
  source-address: 11.0.0.1
  peer-port: 1
  hold-time: 10ns
- peer-address: 20.0.0.2
  peer-asn: 64001
  my-asn: 65001
  router-id: 20.20.20.20
  source-address: 12.0.0.1
  peer-port: 2
  hold-time: 20ns
`))
}
