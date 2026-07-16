package cli

import "testing"

func TestValidateUIPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{name: "root", value: "/", want: "/", ok: true},
		{name: "project", value: "/projects/project_1?tab=terminal", want: "/projects/project_1?tab=terminal", ok: true},
		{name: "workspace", value: "/workspaces?workspace=workspace_1", want: "/workspaces?workspace=workspace_1", ok: true},
		{name: "remote", value: "https://example.com", ok: false},
		{name: "network path", value: "//example.com/path", ok: false},
		{name: "dot segment", value: "/projects/../settings", ok: false},
		{name: "encoded dot segment", value: "/projects/%2e%2e/settings", ok: false},
		{name: "fragment", value: "/projects/a#token", ok: false},
		{name: "credential", value: "/?bootstrap=attacker", ok: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateUIPath(test.value)
			if test.ok && (err != nil || got != test.want) {
				t.Fatalf("validateUIPath(%q) = %q, %v; want %q", test.value, got, err, test.want)
			}
			if !test.ok && err == nil {
				t.Fatalf("validateUIPath(%q) = %q, want error", test.value, got)
			}
		})
	}
}
