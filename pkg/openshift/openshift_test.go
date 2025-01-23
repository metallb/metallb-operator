package openshift

import (
	"testing"

	"github.com/metallb/metallb-operator/pkg/params"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

func TestCnoSupportsFRRK8s(t *testing.T) {
	tests := []struct {
		name      string
		cno       *openshiftconfigv1.ClusterOperator
		expected  bool
		shouldErr bool
	}{
		{
			name: "old version",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "operator",
							Version: "4.16.0",
						},
					},
				},
			},
			expected: false,
		}, {
			name: "exact same version",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "operator",
							Version: "4.17.0",
						},
					},
				},
			},
			expected: true,
		}, {
			name: "greater version",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "operator",
							Version: "4.17.5",
						},
					},
				},
			},
			expected: true,
		}, {
			name: "invalid version",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "operator",
							Version: "4.17.5.6.7.8.9.fooo",
						},
					},
				},
			},
			shouldErr: true,
		}, {
			name: "no operator version",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "hello",
							Version: "4.17.5.6.7.8.9.fooo",
						}, {
							Name:    "world",
							Version: "4.17.5.6.7.8.9.fooo",
						},
					},
				},
			},
			shouldErr: true,
		},
		{
			name: "nightly",
			cno: &openshiftconfigv1.ClusterOperator{
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "hello",
							Version: "4.17.5.6.7.8.9.fooo",
						}, {
							Name:    "operator",
							Version: "4.17.0-0.nightly-2024-07-17-18340",
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envConfig := params.EnvConfig{CNOMinFRRK8sVersion: "4.17.0-0"}
			supports, err := cnoSupportsFRRK8s(test.cno, envConfig)
			if test.shouldErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !test.shouldErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if supports != test.expected {
				t.Fatalf("supports %v different from expected: %v", supports, test.expected)
			}
		})
	}
}
