package openshift

import (
	"crypto/tls"
	"slices"
	"strings"
	"testing"

	"github.com/metallb/metallb-operator/pkg/tlsconfig"
	configv1 "github.com/openshift/api/config/v1"
)

func TestConfigFromProfile_ResolvedTLSSettings(t *testing.T) {
	tests := []struct {
		name        string
		spec        configv1.TLSProfileSpec
		ciphers     string
		curves      string
		version     string
		unsupported []string
	}{
		{
			name: "mixed OpenSSL and IANA cipher names",
			spec: configv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				Groups:        []configv1.TLSGroup{configv1.TLSGroupX25519},
				MinTLSVersion: configv1.VersionTLS12,
			},
			ciphers: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			curves:  "29",
			version: "VersionTLS12",
		},
		{
			name: "TLS 1.3 leaves cipher suites unconfigured",
			spec: configv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_AES_128_GCM_SHA256"},
				MinTLSVersion: configv1.VersionTLS13,
			},
			version: "VersionTLS13",
		},
		{
			name: "unsupported ciphers and groups are reported",
			spec: configv1.TLSProfileSpec{
				Ciphers:       []string{"unknown-cipher", "ECDHE-RSA-AES128-GCM-SHA256"},
				Groups:        []configv1.TLSGroup{"unknown-group", configv1.TLSGroupX25519},
				MinTLSVersion: configv1.VersionTLS12,
			},
			ciphers:     "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			curves:      "29",
			version:     "VersionTLS12",
			unsupported: []string{"unknown-cipher", "unknown-group"},
		},
		{
			name:    "omitted minimum version uses the OpenShift default",
			version: "VersionTLS12",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphers, curves, version, unsupported := tlsStringsFromProfile(tt.spec)
			if ciphers != tt.ciphers || curves != tt.curves || version != tt.version {
				t.Fatalf("TLS settings = (%q, %q, %q), want (%q, %q, %q)",
					ciphers, curves, version, tt.ciphers, tt.curves, tt.version)
			}
			if !slices.Equal(unsupported, tt.unsupported) {
				t.Errorf("unsupported entries = %v, want %v", unsupported, tt.unsupported)
			}
			if _, err := tlsconfig.OptFor(ciphers, curves, version); err != nil {
				t.Errorf("profile strings rejected by TLS configuration: %v", err)
			}
		})
	}
}

func TestConfigFromProfile_Groups(t *testing.T) {
	tests := []struct {
		name        string
		groups      []configv1.TLSGroup
		want        string
		curves      []tls.CurveID
		unsupported []string
	}{
		{name: "omitted groups retain Go defaults"},
		{
			name:   "custom groups preserve order",
			groups: []configv1.TLSGroup{configv1.TLSGroupSecP384r1, configv1.TLSGroupX25519, configv1.TLSGroupSecP256r1},
			want:   "24,29,23",
			curves: []tls.CurveID{tls.CurveP384, tls.X25519, tls.CurveP256},
		},
		{
			name: "all supported groups",
			groups: []configv1.TLSGroup{
				configv1.TLSGroupX25519, configv1.TLSGroupSecP256r1,
				configv1.TLSGroupSecP384r1, configv1.TLSGroupSecP521r1,
				configv1.TLSGroupX25519MLKEM768, configv1.TLSGroupSecP256r1MLKEM768,
				configv1.TLSGroupSecP384r1MLKEM1024,
			},
			want: "29,23,24,25,4588,4587,4589",
			curves: []tls.CurveID{
				tls.X25519, tls.CurveP256, tls.CurveP384, tls.CurveP521,
				tls.X25519MLKEM768, tls.SecP256r1MLKEM768, tls.SecP384r1MLKEM1024,
			},
		},
		{
			name:        "unsupported groups are omitted",
			groups:      []configv1.TLSGroup{"unknown", configv1.TLSGroupX25519},
			unsupported: []string{"unknown"},
			want:        "29",
			curves:      []tls.CurveID{tls.X25519},
		},
		{
			name:        "only unsupported groups retain Go defaults",
			groups:      []configv1.TLSGroup{"unknown"},
			unsupported: []string{"unknown"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := configv1.TLSProfileSpec{Groups: tt.groups, MinTLSVersion: configv1.VersionTLS13}
			ciphers, curves, version, unsupported := tlsStringsFromProfile(spec)
			if !slices.Equal(unsupported, tt.unsupported) {
				t.Errorf("unsupported entries = %v, want %v", unsupported, tt.unsupported)
			}
			if curves != tt.want {
				t.Fatalf("curve preferences = %q, want %q", curves, tt.want)
			}
			opt, err := tlsconfig.OptFor(ciphers, curves, version)
			if err != nil {
				t.Fatalf("profile strings rejected by TLS configuration: %v", err)
			}
			cfg := &tls.Config{}
			opt(cfg)
			if !slices.Equal(cfg.CurvePreferences, tt.curves) {
				t.Errorf("TLS curves = %v, want %v", cfg.CurvePreferences, tt.curves)
			}
		})
	}
}

func TestConfigFromProfile_Intermediate(t *testing.T) {
	spec := *configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	cipherSuites, curvePreferences, minVersion, _ := tlsStringsFromProfile(spec)

	if minVersion != string(configv1.VersionTLS12) {
		t.Errorf("expected VersionTLS12, got %s", minVersion)
	}
	if cipherSuites == "" {
		t.Error("expected non-empty cipher suites")
	}
	for _, c := range strings.Split(cipherSuites, ",") {
		if strings.HasPrefix(c, "ECDHE-") || strings.HasPrefix(c, "DHE-") {
			t.Errorf("cipher %q looks like OpenSSL name, expected IANA", c)
		}
	}
	if curvePreferences != "4588,29,23,24" {
		t.Errorf("unexpected curve preferences: %s", curvePreferences)
	}
}

func TestConfigFromProfile_Modern(t *testing.T) {
	spec := *configv1.TLSProfiles[configv1.TLSProfileModernType]
	_, _, minVersion, _ := tlsStringsFromProfile(spec)

	if minVersion != string(configv1.VersionTLS13) {
		t.Errorf("expected VersionTLS13, got %s", minVersion)
	}
}

func TestConfigFromProfile_Custom(t *testing.T) {
	spec := configv1.TLSProfileSpec{
		Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES256-GCM-SHA384"},
		MinTLSVersion: configv1.VersionTLS12,
	}
	cipherSuites, _, minVersion, _ := tlsStringsFromProfile(spec)

	if minVersion != "VersionTLS12" {
		t.Errorf("expected VersionTLS12, got %s", minVersion)
	}
	ciphers := strings.Split(cipherSuites, ",")
	if len(ciphers) != 2 {
		t.Fatalf("expected 2 IANA ciphers, got %d: %s", len(ciphers), cipherSuites)
	}
	if ciphers[0] != "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
		t.Errorf("unexpected first cipher: %s", ciphers[0])
	}
	if ciphers[1] != "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" {
		t.Errorf("unexpected second cipher: %s", ciphers[1])
	}
}

func TestConfigFromProfile_NoCiphers(t *testing.T) {
	spec := configv1.TLSProfileSpec{
		MinTLSVersion: configv1.VersionTLS13,
	}
	cipherSuites, _, _, _ := tlsStringsFromProfile(spec)
	if cipherSuites != "" {
		t.Errorf("expected empty cipher suites, got %s", cipherSuites)
	}
}
