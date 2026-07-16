package cli

import (
	"errors"
	"path/filepath"
	"testing"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func TestSelectProjectByIDSlugAndPath(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	projects := []generated.Project{{Id: "project_alpha", Slug: "alpha", PrimaryLocation: path}, {Id: "project_beta", Slug: "beta", PrimaryLocation: filepath.Join(t.TempDir(), "beta")}}
	for _, selection := range []string{"project_alpha", "alpha", path} {
		project, err := selectProject(projects, selection)
		if err != nil || project.Id != "project_alpha" {
			t.Fatalf("selectProject(%q) = %#v, %v", selection, project, err)
		}
	}
}

func TestSelectProjectReportsStableMissingAndAmbiguousErrors(t *testing.T) {
	t.Parallel()
	projects := []generated.Project{{Id: "one", Slug: "same"}, {Id: "two", Slug: "same"}}
	_, err := selectProject(projects, "same")
	var cliErr *Error
	if !errors.As(err, &cliErr) || cliErr.Code != "PROJECT_AMBIGUOUS" || cliErr.ExitCode != exitConflict {
		t.Fatalf("ambiguous error = %#v", err)
	}
	_, err = selectProject(projects, "missing")
	if !errors.As(err, &cliErr) || cliErr.Code != "PROJECT_NOT_FOUND" || cliErr.ExitCode != exitNotFound {
		t.Fatalf("missing error = %#v", err)
	}
}
