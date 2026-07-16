package domain

import "testing"

func TestLocalRouteValidation(t *testing.T) {
	t.Parallel()

	host, err := HostnameFromRequest("Feature.Project.localhost:18080")
	if err != nil || host != "feature.project.localhost" {
		t.Fatalf("host=%q error=%v", host, err)
	}
	for _, invalid := range []string{"localhost", "project.example.com", "-project.localhost", "project.localhost:bad"} {
		if _, err := HostnameFromRequest(invalid); err == nil {
			t.Errorf("HostnameFromRequest(%q) succeeded", invalid)
		}
	}
	for _, valid := range []string{"http://127.0.0.1:8080", "http://[::1]:8080/base", "http://localhost:3000"} {
		if _, err := ValidateTarget(valid); err != nil {
			t.Errorf("ValidateTarget(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{
		"https://127.0.0.1:8080", "http://example.com", "http://user:secret@localhost:3000",
		"http://localhost:3000?token=secret", "/relative",
	} {
		if _, err := ValidateTarget(invalid); err == nil {
			t.Errorf("ValidateTarget(%q) succeeded", invalid)
		}
	}
}
