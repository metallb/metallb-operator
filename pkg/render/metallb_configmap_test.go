package render_test

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/metallb/metallb-operator/pkg/render"
)

func TestRendering(t *testing.T) {
	tests := map[string]render.OperatorConfig{
		"poolRendering": {
			ConfigMapName: "config",
			NameSpace:     "namespace",
			Pools: []metallbv1alpha1.AddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool1",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1-1.1.1.100",
						},
						AutoAssign: pointer.BoolPtr(false),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool2",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.2-2.2.2.100",
							"2.2.3.2-2.2.3.100",
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool3",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.2-2.2.2.100",
							"2.2.3.2-2.2.3.100",
						},
						AutoAssign: pointer.BoolPtr(true),
					},
				},
			},
			Peers: []metallbv1alpha1.BGPPeer{},
		},
		"communitiesRendering": {
			ConfigMapName: "config",
			NameSpace:     "namespace",
			Pools: []metallbv1alpha1.AddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool1",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "bgp",
						Addresses: []string{
							"1.1.1.1-1.1.1.100",
						},
						AutoAssign: pointer.BoolPtr(false),
						BGPAdvertisements: []metallbv1alpha1.BgpAdvertisement{
							{
								AggregationLength:   pointer.Int32Ptr(57),
								AggregationLengthV6: pointer.Int32Ptr(64),
								LocalPref:           42,
								Communities: []string{
									"foo",
									"bar",
								},
							}, {
								AggregationLength:   pointer.Int32Ptr(58),
								AggregationLengthV6: pointer.Int32Ptr(120),
								LocalPref:           43,
								Communities: []string{
									"foo1",
									"bar1",
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool2",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "bgp",
						Addresses: []string{
							"2.2.2.2-2.2.2.100",
							"2.2.3.2-2.2.3.100",
						},
					},
				},
			},
			Peers: []metallbv1alpha1.BGPPeer{},
		},
		"peersRendering": {
			ConfigMapName: "config",
			NameSpace:     "namespace",
			Pools: []metallbv1alpha1.AddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-addresspool1",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "bgp",
						Addresses: []string{
							"1.1.1.1-1.1.1.100",
						},
					},
				},
			},
			Peers: []metallbv1alpha1.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-peer1",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.BGPPeerSpec{
						MyASN:      23,
						ASN:        24,
						Address:    "192.168.1.1",
						SrcAddress: "192.168.1.2",
						Port:       1234,
						HoldTime:   time.Second,
						RouterID:   "abcd",
						NodeSelectors: []metallbv1alpha1.NodeSelector{
							{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
								MatchExpressions: []metallbv1alpha1.MatchExpression{

									{

										Key:      "k1",
										Operator: "op1",
										Values:   []string{"val1", "val2", "val3"},
									},
								},
							}, {
								MatchLabels: map[string]string{
									"foo1": "bar1",
								},
							},
						},
						Password: "topsecret",
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-peer2",
						Namespace: "namespace",
					},
					Spec: metallbv1alpha1.BGPPeerSpec{
						MyASN:      25,
						ASN:        26,
						Address:    "192.168.2.1",
						SrcAddress: "192.168.2.2",
					},
				},
			},
		}, "empty": {
			ConfigMapName: "config",
			NameSpace:     "namespace",
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			data.DataField = "config"
			cm, err := render.OperatorConfigToMetalLB(&data)
			if err != nil {
				t.Fatalf("Failed to render %s %v", name, err)
			}
			closer := dump(cm.Data["config"], t)
			checkRendered(t)
			err = closer()
			if err != nil {
				t.Fatalf("Failed to close %s %v", name, err)
			}
		})
	}
}

var update = flag.Bool("update", false, "update .golden files")

func dump(config string, t *testing.T) func() error {
	configFile, _ := renderedNames(t)

	cmd := exec.Command("rm", configFile)
	_, _ = cmd.Output() // ignoring failed deletions

	f, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("cannot create config file %s: %s", configFile, err)
	}
	_, err = f.Write([]byte(config))
	if err != nil {
		t.Fatalf("cannot write config file %s: %s", configFile, err)
	}
	return f.Close
}

func checkRendered(t *testing.T) {
	configFile, goldenFile := renderedNames(t)

	if *update {
		updateGolden(t, configFile, goldenFile)
	}
	compareRendered(t, configFile, goldenFile)
}

func compareRendered(t *testing.T, configFile, goldenFile string) {
	cmd := exec.Command("diff", configFile, goldenFile)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("command %s returned error: %s\n%s", cmd.String(), err, output)
	}
}

func updateGolden(t *testing.T, configFile, goldenFile string) {
	t.Log("update golden file")
	cmd := exec.Command("cp", "-a", configFile, goldenFile)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("command %s returned %s and error: %s", cmd.String(), output, err)
	}
}

func renderedNames(t *testing.T) (string, string) {
	return filepath.Join("testdata", filepath.FromSlash(t.Name())), filepath.Join("testdata", filepath.FromSlash(t.Name())+".golden")
}
