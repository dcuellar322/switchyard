package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
	"switchyard.dev/switchyard/internal/transport/httpclient"
)

func newProjectCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "project", Aliases: []string{"projects"}, Short: "Manage the local project catalog"}
	command.AddCommand(newProjectListCommand(options), newProjectGetCommand(options), newAddCommand(options), newProjectTrustCommand(options), newProjectRemoveCommand(options))
	return command
}

func newListAliasCommand(options *rootOptions) *cobra.Command {
	command := newProjectListCommand(options)
	command.Use = "list"
	command.Aliases = nil
	return command
}

func newProjectListCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List registered projects", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		projects, err := client.Projects(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "project.list", projects, func(w io.Writer) error {
			rows := make([][]string, 0, len(projects))
			for _, project := range projects {
				rows = append(rows, []string{project.Slug, string(project.TrustState), fmt.Sprint(project.ManifestRevision), project.PrimaryLocation})
			}
			return humanList(w, []string{"PROJECT", "TRUST", "REVISION", "LOCATION"}, rows)
		})
	}}
}

func newProjectGetCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "get <project>", Short: "Read a project by ID, unique slug, or path", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "project.get", project, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s (%s)\nid: %s\ntrust: %s\nrevision: %d\nlocation: %s\ntags: %s\n", project.DisplayName, project.Slug, project.Id, project.TrustState, project.ManifestRevision, project.PrimaryLocation, strings.Join(project.Tags, ", "))
			return err
		})
	}}
}

func newAddCommand(options *rootOptions) *cobra.Command {
	allowOutsideRoots := false
	command := &cobra.Command{Use: "add <repository>", Short: "Scan a repository and create a reviewable proposal", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		proposal, err := client.CreateManifestProposalWithRootOverride(command.Context(), args[0], key, allowOutsideRoots)
		if err != nil {
			return err
		}
		return writeResult(options, "project.add", proposal, func(w io.Writer) error {
			name := proposal.Id
			if metadata, ok := proposal.Candidate["metadata"].(map[string]any); ok {
				if candidateName, ok := metadata["name"].(string); ok {
					name = candidateName
				}
			}
			_, err := fmt.Fprintf(w, "proposal %s created for %s\nproject: %s\nevidence: %d\nvalid: %t\nunresolved: %s\n", proposal.Id, name, proposal.ProjectId, len(proposal.Evidence), proposal.Validation.Valid, strings.Join(proposal.Unresolved, ", "))
			return err
		})
	}}
	command.Flags().BoolVar(&allowOutsideRoots, "allow-outside-roots", false, "explicitly approve this one scan outside configured project roots")
	return command
}

func newProjectTrustCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "trust <project>", Short: "Approve the latest valid project proposal", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("CONFIRMATION_REQUIRED", "project trust requires --yes after reviewing its evidence")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		accepted, err := client.TrustProject(command.Context(), project.Id, key)
		if err != nil {
			return err
		}
		return writeResult(options, "project.trust", accepted, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "trusted %s at manifest revision %d\n", accepted.Project.Slug, accepted.Project.ManifestRevision)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the trust decision")
	return command
}

func newProjectRemoveCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "remove <project>", Short: "Remove catalog state without deleting repository files", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("CONFIRMATION_REQUIRED", "project remove requires --yes; repository files are never deleted")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		project, err := resolveProject(command.Context(), client, args[0])
		if err != nil {
			return err
		}
		key, err := identifier.New("cli")
		if err != nil {
			return err
		}
		if err := client.RemoveProject(command.Context(), project.Id, key); err != nil {
			return err
		}
		result := map[string]any{"id": project.Id, "slug": project.Slug, "removed": true, "repositoryFilesChanged": false}
		return writeResult(options, "project.remove", result, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "removed %s from the catalog; repository files were not changed\n", project.Slug)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm catalog removal")
	return command
}

func resolveProject(ctx context.Context, client *httpclient.Client, value string) (generated.Project, error) {
	projects, err := client.Projects(ctx)
	if err != nil {
		return generated.Project{}, err
	}
	return selectProject(projects, value)
}

func selectProject(projects []generated.Project, value string) (generated.Project, error) {
	canonical := canonicalSelectionPath(value)
	matches := make([]generated.Project, 0, 1)
	for _, project := range projects {
		if project.Id == value {
			return project, nil
		}
		if project.Slug == value || canonical != "" && canonical == canonicalSelectionPath(project.PrimaryLocation) {
			matches = append(matches, project)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, project := range matches {
			ids = append(ids, project.Id)
		}
		slices.Sort(ids)
		return generated.Project{}, conflictError("PROJECT_AMBIGUOUS", fmt.Sprintf("project %q matches multiple IDs: %s", value, strings.Join(ids, ", ")))
	}
	return generated.Project{}, notFoundError(fmt.Sprintf("project %q was not found by ID, slug, or path", value))
}

func canonicalSelectionPath(value string) string {
	if value == "" || !strings.ContainsAny(value, `/\\`) && value != "." {
		return ""
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return ""
	}
	if canonical, err := filepath.EvalSymlinks(absolute); err == nil {
		return canonical
	}
	return filepath.Clean(absolute)
}
