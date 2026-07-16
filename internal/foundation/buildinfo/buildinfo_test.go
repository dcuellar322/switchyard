package buildinfo

import "testing"

func TestCurrentHasDevelopmentDefaults(t *testing.T) {
	t.Parallel()

	info := Current()
	if info.Version == "" || info.Commit == "" {
		t.Fatalf("expected non-empty build identity, got %#v", info)
	}
}
