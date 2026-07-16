package domain

import "testing"

func TestParseActionAcceptsOnlyLifecycleContract(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"start", "stop", "restart", "pause", "unpause", "rebuild", "teardown"} {
		if action, err := ParseAction(value); err != nil || string(action) != value {
			t.Fatalf("ParseAction(%q) = %q, %v", value, action, err)
		}
	}
	if _, err := ParseAction("exec"); err == nil {
		t.Fatal("ParseAction(exec) succeeded")
	}
}
