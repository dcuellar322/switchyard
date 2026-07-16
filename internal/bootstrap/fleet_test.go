package bootstrap

import (
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

func TestParseRemoteControllersRequiresExplicitUniqueInventoryGrants(t *testing.T) {
	t.Parallel()
	fingerprint := strings.Repeat("a", 64)
	grants, err := parseRemoteControllers([]string{fingerprint + "=inventory.read,project.operate"})
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || len(grants[0].Capabilities) != 2 || grants[0].Capabilities[1] != domain.CapabilityProjectOperate {
		t.Fatalf("grants = %#v", grants)
	}
	for _, value := range [][]string{
		{},
		{fingerprint + "=project.operate"},
		{fingerprint + "=inventory.read,unknown"},
		{fingerprint + "=inventory.read", fingerprint + "=inventory.read"},
	} {
		if _, err := parseRemoteControllers(value); err == nil {
			t.Fatalf("parseRemoteControllers(%q) error = nil", value)
		}
	}
}

func TestValidateRemoteConfigKeepsLocalOnlyDefault(t *testing.T) {
	t.Parallel()
	if err := validateRemoteConfig(Config{}); err != nil {
		t.Fatalf("local-only config error = %v", err)
	}
	if err := validateRemoteConfig(Config{RemoteMachineID: "unexpected"}); err == nil {
		t.Fatal("partial remote config error = nil")
	}
}
