package test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestReleaseChannelClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tag     string
		channel string
		valid   bool
	}{
		{tag: "v1.0.0", channel: "stable", valid: true},
		{tag: "v1.1.0-alpha.1", channel: "alpha", valid: true},
		{tag: "v1.1.0-beta.2", channel: "beta", valid: true},
		{tag: "v1.2.0-nightly.20260716", channel: "nightly", valid: true},
		{tag: "1.0.0", valid: false},
		{tag: "v1.0", valid: false},
		{tag: "v1.0.0-rc.1", valid: false},
		{tag: "v1.0.0;echo-unsafe", valid: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.tag, func(t *testing.T) {
			t.Parallel()
			command := exec.Command("sh", "../scripts/release-channel.sh", test.tag)
			output, err := command.CombinedOutput()
			if !test.valid {
				if err == nil {
					t.Fatalf("accepted invalid release tag; output=%q", output)
				}
				return
			}
			if err != nil {
				t.Fatalf("classify release tag: %v: %s", err, output)
			}
			if strings.TrimSpace(string(output)) != test.channel {
				t.Fatalf("channel = %q, want %q", output, test.channel)
			}
		})
	}
}
