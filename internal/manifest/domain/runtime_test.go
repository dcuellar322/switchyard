package domain

import (
	"strings"
	"testing"
)

func TestComposeProfilesMustBeUniqueAndNonEmpty(t *testing.T) {
	t.Parallel()
	problems := validateRuntime(Runtime{Driver: "compose", Compose: &ComposeConfig{
		Files: []string{"compose.yaml"}, Profiles: []string{"marketing", "", "marketing"},
	}})
	if len(problems) != 2 {
		t.Fatalf("problems = %v", problems)
	}
	message := problems[0].Error() + " " + problems[1].Error()
	if !strings.Contains(message, "empty name") || !strings.Contains(message, "more than once") {
		t.Fatalf("problems = %v", problems)
	}
}
