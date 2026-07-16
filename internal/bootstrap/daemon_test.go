package bootstrap

import "testing"

func TestValidateLoopbackAddress(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"127.0.0.1:19616", "[::1]:19616", "localhost:19616"} {
		if err := validateLoopbackAddress(address); err != nil {
			t.Errorf("validateLoopbackAddress(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19616", "192.0.2.1:19616", "missing-port"} {
		if err := validateLoopbackAddress(address); err == nil {
			t.Errorf("validateLoopbackAddress(%q) error = nil", address)
		}
	}
}
