package v1alpha1

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetalLBTestNameSpace = "metallb-test-namespace"
)

func TestValidateAddressPool(t *testing.T) {
	autoAssign := false
	addressPool := AddressPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addresspool",
			Namespace: MetalLBTestNameSpace,
		},
		Spec: AddressPoolSpec{
			Protocol: "layer2",
			Addresses: []string{
				"1.1.1.1-1.1.1.100",
			},
			AutoAssign: &autoAssign,
		},
	}
	addressPoolList := &AddressPoolList{}
	addressPoolList.Items = append(addressPoolList.Items, addressPool)

	tests := []struct {
		desc             string
		addressPool      *AddressPool
		isNewAddressPool bool
		expectedError    string
	}{
		{
			desc: "Second AddressPool, already defined name",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.101-1.1.1.200",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "duplicate definition of pool",
		},
		{
			desc: "Second AddressPool, overlapping addresses defined by address range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.15-1.1.1.20",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "overlaps with already defined CIDR",
		},
		{
			desc: "Second AddressPool, overlapping addresses defined by network prefix",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.0/24",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "overlaps with already defined CIDR",
		},
		{
			desc: "Second AddressPool, invalid CIDR, single address provided while expecting a range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.15",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid CIDR",
		},
		{
			desc: "Second AddressPool, invalid CIDR, first address of the range is after the second",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.200-1.1.1.101",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid IP range",
		},
		{
			desc: "Second AddressPool, invalid ipv6 CIDR, single address provided while expecting a range",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2000::",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid CIDR",
		},
		{
			desc: "Second AddressPool, invalid ipv6 CIDR, first address of the range is after the second",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2000:::ffff-2000::",
					},
					AutoAssign: &autoAssign,
				},
			},
			isNewAddressPool: true,
			expectedError:    "invalid IP range",
		},
		{
			desc: "Invalid protocol used while using bgp advertisments",
			addressPool: &AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2.2.2.2-2.2.2.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []BgpAdvertisement{
						{
							AggregationLength: 24,
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			},
			isNewAddressPool: true,
			expectedError:    "bgpadvertisement config not valid",
		},
	}

	for _, test := range tests {
		err := test.addressPool.validateAddressPool(test.isNewAddressPool, addressPoolList)
		if err == nil {
			t.Errorf("%s: ValidateAddressPool failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateAddressPool failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}
