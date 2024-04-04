package v1beta1

import (
	"errors"
	"testing"
)

func TestValidateFRRK8sConfig(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		err := validateFRRK8sConfig(nil)
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
	})

	t.Run("ValidConfig", func(t *testing.T) {
		config := &FRRK8SConfig{
			AlwaysBlock: []string{"192.168.0.0/24", "10.0.0.0/16"},
		}
		err := validateFRRK8sConfig(config)
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
	})

	t.Run("InvalidCIDR", func(t *testing.T) {
		config := &FRRK8SConfig{
			AlwaysBlock: []string{"192.168.0.0/24", "invalid_cidr"},
		}
		err := validateFRRK8sConfig(config)
		expectedErr := errors.New("invalid CIDR invalid_cidr in AlwaysBlock")
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("Expected error: %v, got: %v", expectedErr, err)
		}
	})

	t.Run("ValidIPv6Config", func(t *testing.T) {
		config := &FRRK8SConfig{
			AlwaysBlock: []string{"2001:db8::/32", "2001:db8:85a3::/48"},
		}
		err := validateFRRK8sConfig(config)
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
	})
}
