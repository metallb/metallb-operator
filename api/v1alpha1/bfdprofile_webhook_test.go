package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateBFDProfile(t *testing.T) {
	bfdProfile := BFDProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bfdprofile",
			Namespace: MetalLBTestNameSpace,
		},
	}
	bfdProfileList := &BFDProfileList{}
	bfdProfileList.Items = append(bfdProfileList.Items, bfdProfile)

	tests := []struct {
		desc            string
		bfdProfile      *BFDProfile
		isNewBFDProfile bool
		expectedError   string
	}{
		{
			desc: "new profile with already defined name",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewBFDProfile: true,
			expectedError:   "duplicate definition of bfdprofile",
		},
		{
			desc: "new profile, missing name",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: MetalLBTestNameSpace,
				},
			},
			isNewBFDProfile: true,
			expectedError:   "missing bfdprofile name",
		},
		{
			desc: "new profile with invalid detect multiplier value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					DetectMultiplier: uint32Ptr(BFDMaxDetectMultiplier + 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid detect multiplier value",
		},
		{
			desc: "new profile with invalid detect multiplier value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					DetectMultiplier: uint32Ptr(BFDMinDetectMultiplier - 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid detect multiplier value",
		}, {
			desc: "update profile with invalid detect multiplier value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					DetectMultiplier: uint32Ptr(BFDMaxDetectMultiplier + 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid detect multiplier value",
		},
		{
			desc: "update profile with invalid detect multiplier value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					DetectMultiplier: uint32Ptr(BFDMinDetectMultiplier - 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid detect multiplier value",
		},

		{
			desc: "new profile with invalid receive interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMaxReceiveInterval + 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid receive interval value",
		},
		{
			desc: "new profile with invalid receive interval value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMinReceiveInterval - 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid receive interval value",
		},
		{
			desc: "update profile with invalid receive interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMaxReceiveInterval + 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid receive interval value",
		},
		{
			desc: "update profile with invalid receive interval value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMinReceiveInterval - 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid receive interval value",
		},

		{
			desc: "new profile with invalid transmit interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMaxTransmitInterval + 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid transmit interval value",
		},
		{
			desc: "new profile with invalid transmit interval value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMinTransmitInterval - 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid transmit interval value",
		},
		{
			desc: "update profile with invalid transmit interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMaxTransmitInterval + 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid transmit interval value",
		},
		{
			desc: "update profile with invalid transmit interval value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMinTransmitInterval - 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid transmit interval value",
		},

		{
			desc: "new profile with invalid minimum ttl value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					MinimumTTL: uint32Ptr(BFDMaxMinimumTTL + 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid minimum ttl value",
		},
		{
			desc: "new profile with invalid minimum ttl value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					MinimumTTL: uint32Ptr(BFDMinMinimumTTL - 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid minimum ttl value",
		},
		{
			desc: "update profile with invalid minimum ttl value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMaxMinimumTTL + 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid minimum ttl value",
		},
		{
			desc: "update profile with invalid minimum ttl value (under minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					ReceiveInterval: uint32Ptr(BFDMinMinimumTTL - 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid minimum ttl value",
		},

		{
			desc: "new profile with invalid echo receive interval value (invalid string)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr("bad"),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "new profile with invalid echo receive interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr(strconv.Itoa(BFDMaxEchoReceiveInterval + 1)),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "new profile with invalid echo receive interval value (under  minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr(strconv.Itoa(BFDMinEchoReceiveInterval + 1)),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "update profile with invalid echo receive interval value (invalid string)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr("bad"),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "update profile with invalid echo receive interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr(strconv.Itoa(BFDMaxEchoReceiveInterval + 1)),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "update profile with invalid echo receive interval value (under  minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoReceiveInterval: pointer.StringPtr(strconv.Itoa(BFDMinEchoReceiveInterval - 1)),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid echo receive interval value",
		},
		{
			desc: "new profile with invalid echo transmit interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoTransmitInterval: uint32Ptr(BFDMaxEchoTransmitInterval + 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid echo transmit interval value",
		},
		{
			desc: "new profile with invalid echo transmit interval value (under  minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile-new",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoTransmitInterval: uint32Ptr(BFDMinEchoTransmitInterval - 1),
				},
			},
			isNewBFDProfile: true,
			expectedError:   "invalid echo transmit interval value",
		},
		{
			desc: "update profile with invalid echo transmit interval value (over maximum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoTransmitInterval: uint32Ptr(BFDMaxEchoTransmitInterval + 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid echo transmit interval value",
		},
		{
			desc: "update profile with invalid echo transmit value (under  minimum limit)",
			bfdProfile: &BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bfdprofile",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: BFDProfileSpec{
					EchoTransmitInterval: uint32Ptr(BFDMinEchoTransmitInterval - 1),
				},
			},
			isNewBFDProfile: false,
			expectedError:   "invalid echo transmit interval value",
		},
	}

	for _, test := range tests {
		err := test.bfdProfile.validateBFDProfile(test.isNewBFDProfile, bfdProfileList)
		if err == nil {
			t.Errorf("%s: ValidateBFDProfile failed, no error found while expected: \"%s\"", test.desc, test.expectedError)
		} else {
			if !strings.Contains(fmt.Sprint(err), test.expectedError) {
				t.Errorf("%s: ValidateBFDProfile failed, expected error: \"%s\" to contain: \"%s\"", test.desc, err, test.expectedError)
			}
		}
	}
}

func uint32Ptr(n uint32) *uint32 {
	return &n
}
