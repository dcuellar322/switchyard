package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func TestResolveRepositoryPathUsesClientWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	resolved, err := resolveRepositoryPath(".")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != root {
		t.Fatalf("resolveRepositoryPath(.) = %q, want %q", resolved, root)
	}
}

func TestAddSummaryDistinguishesAcceptedProposalAndEmptyUnresolvedFields(t *testing.T) {
	var output bytes.Buffer
	proposal := generated.ManifestProposal{
		Id: "proposal-1", ProjectId: "project-1", Status: generated.ManifestProposalStatusAccepted,
		Candidate: map[string]any{"metadata": map[string]any{"name": "Switchyard"}},
		Evidence:  []generated.DiscoveryEvidence{}, Unresolved: []string{}, Validation: generated.ManifestValidation{Valid: true},
	}
	if err := writeAddSummary(&output, proposal); err != nil {
		t.Fatal(err)
	}
	if value := output.String(); !strings.Contains(value, "is already accepted for Switchyard") || !strings.Contains(value, "unresolved: none") {
		t.Fatalf("add summary = %q", value)
	}
}

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
