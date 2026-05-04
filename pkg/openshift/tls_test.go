package openshift

import (
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestConfigFromProfile_Intermediate(t *testing.T) {
	spec := *configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	cipherSuites, curvePreferences, minVersion := tlsStringsFromProfile(spec)

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
	if curvePreferences != "" {
		t.Errorf("expected empty curve preferences (deferred), got %s", curvePreferences)
	}
}

func TestConfigFromProfile_Modern(t *testing.T) {
	spec := *configv1.TLSProfiles[configv1.TLSProfileModernType]
	_, _, minVersion := tlsStringsFromProfile(spec)

	if minVersion != string(configv1.VersionTLS13) {
		t.Errorf("expected VersionTLS13, got %s", minVersion)
	}
}

func TestConfigFromProfile_Custom(t *testing.T) {
	spec := configv1.TLSProfileSpec{
		Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES256-GCM-SHA384"},
		MinTLSVersion: configv1.VersionTLS12,
	}
	cipherSuites, _, minVersion := tlsStringsFromProfile(spec)

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
	cipherSuites, _, _ := tlsStringsFromProfile(spec)
	if cipherSuites != "" {
		t.Errorf("expected empty cipher suites, got %s", cipherSuites)
	}
}
