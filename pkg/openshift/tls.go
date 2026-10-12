package openshift

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	crctls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLSConfig holds the resolved OpenShift TLS profile and adherence policy.
type TLSConfig struct {
	CipherSuites     string // IANA names, comma-separated
	CurvePreferences string // numeric CurveIDs, comma-separated
	MinVersion       string // e.g. "VersionTLS12"

	profileSpec configv1.TLSProfileSpec
	adherence   configv1.TLSAdherencePolicy
}

// FetchTLSConfig fetches the cluster-wide TLS security profile and adherence
// policy from apiservers.config.openshift.io/cluster and returns the resolved
// TLS parameters as plain strings.
func FetchTLSConfig(ctx context.Context, scheme *runtime.Scheme, logger logr.Logger) (*TLSConfig, error) {
	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	profileSpec, err := crctls.FetchAPIServerTLSProfile(ctx, cl)
	if err != nil {
		return nil, err
	}

	adherence, err := crctls.FetchAPIServerTLSAdherencePolicy(ctx, cl)
	if err != nil {
		return nil, err
	}

	cipherSuites, curvePreferences, minVersion, unsupported := tlsStringsFromProfile(profileSpec)
	if len(unsupported) > 0 {
		logger.Info("ignoring unsupported TLS profile entries", "entries", unsupported)
	}
	logger.Info("resolved OpenShift TLS profile",
		"ciphers", cipherSuites, "curvePreferences", curvePreferences, "minVersion", minVersion)

	return &TLSConfig{
		CipherSuites:     cipherSuites,
		CurvePreferences: curvePreferences,
		MinVersion:       minVersion,
		profileSpec:      profileSpec,
		adherence:        adherence,
	}, nil
}

// SetupProfileWatcher registers a controller that watches the APIServer CR for
// TLS profile or adherence policy changes and calls cancel() to trigger a
// graceful operator restart.
func (tc *TLSConfig) SetupProfileWatcher(mgr ctrl.Manager, cancel context.CancelFunc, logger logr.Logger) error {
	watcher := &crctls.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     tc.profileSpec,
		InitialTLSAdherencePolicy: tc.adherence,
		OnProfileChange: func(_ context.Context, _, _ configv1.TLSProfileSpec) {
			logger.Info("TLS profile changed, restarting operator")
			cancel()
		},
		OnAdherencePolicyChange: func(_ context.Context, _, _ configv1.TLSAdherencePolicy) {
			logger.Info("TLS adherence policy changed, restarting operator")
			cancel()
		},
	}
	return watcher.SetupWithManager(mgr)
}

func tlsStringsFromProfile(spec configv1.TLSProfileSpec) (string, string, string, []string) {
	configureTLS, unsupported := crctls.NewTLSConfigFromProfile(spec)
	cfg := &tls.Config{}
	configureTLS(cfg)
	ciphers := make([]string, len(cfg.CipherSuites))
	for i, id := range cfg.CipherSuites {
		ciphers[i] = tls.CipherSuiteName(id)
	}
	cipherSuites := strings.Join(ciphers, ",")
	curves := make([]string, len(cfg.CurvePreferences))
	for i, id := range cfg.CurvePreferences {
		curves[i] = strconv.FormatInt(int64(id), 10)
	}
	curvePreferences := strings.Join(curves, ",")
	minVersion := libgocrypto.TLSVersionToNameOrDie(cfg.MinVersion)
	return cipherSuites, curvePreferences, minVersion, unsupported
}
