package openshift

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metallb/metallb-operator/pkg/tlsconfig"
	configv1 "github.com/openshift/api/config/v1"
)

func TestTLSStringsFromProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           configv1.TLSProfileSpec
		wantCiphers     string
		wantCurves      string
		wantVersion     string
		wantUnsupported []string
		wantCurveIDs    []tls.CurveID
	}{
		{
			name: "mixed OpenSSL and IANA cipher names",
			input: configv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				Groups:        []configv1.TLSGroup{configv1.TLSGroupX25519},
				MinTLSVersion: configv1.VersionTLS12,
			},
			wantCiphers:  "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			wantCurves:   "29",
			wantVersion:  "VersionTLS12",
			wantCurveIDs: []tls.CurveID{tls.X25519},
		},
		{
			name: "TLS 1.3 leaves cipher suites unconfigured",
			input: configv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_AES_128_GCM_SHA256"},
				MinTLSVersion: configv1.VersionTLS13,
			},
			wantVersion: "VersionTLS13",
		},
		{
			name: "unsupported ciphers and groups are reported",
			input: configv1.TLSProfileSpec{
				Ciphers:       []string{"unknown-cipher", "ECDHE-RSA-AES128-GCM-SHA256"},
				Groups:        []configv1.TLSGroup{"unknown-group", configv1.TLSGroupX25519},
				MinTLSVersion: configv1.VersionTLS12,
			},
			wantCiphers:     "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			wantCurves:      "29",
			wantVersion:     "VersionTLS12",
			wantUnsupported: []string{"unknown-cipher", "unknown-group"},
			wantCurveIDs:    []tls.CurveID{tls.X25519},
		},
		{
			name:        "omitted minimum version uses the OpenShift default",
			wantVersion: "VersionTLS12",
		},
		{
			name:        "omitted groups retain Go defaults",
			input:       configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS13},
			wantVersion: "VersionTLS13",
		},
		{
			name: "custom groups preserve order",
			input: configv1.TLSProfileSpec{
				Groups:        []configv1.TLSGroup{configv1.TLSGroupSecP384r1, configv1.TLSGroupX25519, configv1.TLSGroupSecP256r1},
				MinTLSVersion: configv1.VersionTLS13,
			},
			wantCurves:   "24,29,23",
			wantVersion:  "VersionTLS13",
			wantCurveIDs: []tls.CurveID{tls.CurveP384, tls.X25519, tls.CurveP256},
		},
		{
			name: "all supported groups",
			input: configv1.TLSProfileSpec{
				Groups: []configv1.TLSGroup{
					configv1.TLSGroupX25519, configv1.TLSGroupSecP256r1,
					configv1.TLSGroupSecP384r1, configv1.TLSGroupSecP521r1,
					configv1.TLSGroupX25519MLKEM768, configv1.TLSGroupSecP256r1MLKEM768,
					configv1.TLSGroupSecP384r1MLKEM1024,
				},
				MinTLSVersion: configv1.VersionTLS13,
			},
			wantCurves:  "29,23,24,25,4588,4587,4589",
			wantVersion: "VersionTLS13",
			wantCurveIDs: []tls.CurveID{
				tls.X25519, tls.CurveP256, tls.CurveP384, tls.CurveP521,
				tls.X25519MLKEM768, tls.SecP256r1MLKEM768, tls.SecP384r1MLKEM1024,
			},
		},
		{
			name: "unsupported groups are omitted",
			input: configv1.TLSProfileSpec{
				Groups:        []configv1.TLSGroup{"unknown", configv1.TLSGroupX25519},
				MinTLSVersion: configv1.VersionTLS13,
			},
			wantCurves:      "29",
			wantVersion:     "VersionTLS13",
			wantUnsupported: []string{"unknown"},
			wantCurveIDs:    []tls.CurveID{tls.X25519},
		},
		{
			name: "only unsupported groups retain Go defaults",
			input: configv1.TLSProfileSpec{
				Groups:        []configv1.TLSGroup{"unknown"},
				MinTLSVersion: configv1.VersionTLS13,
			},
			wantVersion:     "VersionTLS13",
			wantUnsupported: []string{"unknown"},
		},
		{
			name:  "intermediate profile",
			input: *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
			wantCiphers: strings.Join([]string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
			}, ","),
			wantCurves:   "4588,29,23,24",
			wantVersion:  "VersionTLS12",
			wantCurveIDs: []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384},
		},
		{
			name:         "modern profile",
			input:        *configv1.TLSProfiles[configv1.TLSProfileModernType],
			wantCurves:   "4588,29,23,24",
			wantVersion:  "VersionTLS13",
			wantCurveIDs: []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384},
		},
		{
			name: "custom profile",
			input: configv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES256-GCM-SHA384"},
				MinTLSVersion: configv1.VersionTLS12,
			},
			wantCiphers: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			wantVersion: "VersionTLS12",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ciphers, curves, version, unsupported := tlsStringsFromProfile(tt.input)
			if ciphers != tt.wantCiphers || curves != tt.wantCurves || version != tt.wantVersion {
				t.Errorf("TLS settings = (%q, %q, %q), want (%q, %q, %q)",
					ciphers, curves, version, tt.wantCiphers, tt.wantCurves, tt.wantVersion)
			}
			if diff := cmp.Diff(tt.wantUnsupported, unsupported, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unsupported mismatch (-want +got):\n%s", diff)
			}
			opt, err := tlsconfig.OptFor(ciphers, curves, version)
			if err != nil {
				t.Fatalf("profile strings rejected by TLS configuration: %v", err)
			}
			cfg := &tls.Config{}
			opt(cfg)
			if diff := cmp.Diff(tt.wantCurveIDs, cfg.CurvePreferences, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("TLS curves mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
