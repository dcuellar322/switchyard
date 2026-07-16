package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestWorkspaceTopologicalLayersOrdersDependenciesAndParallelBranches(t *testing.T) {
	t.Parallel()

	workspace := fixtureWorkspace()
	layers, err := workspace.TopologicalLayers(nil)
	if err != nil {
		t.Fatalf("TopologicalLayers() error = %v", err)
	}
	want := [][]string{{"database", "cache"}, {"api", "worker"}, {"web"}}
	if !reflect.DeepEqual(layers, want) {
		t.Fatalf("TopologicalLayers() = %#v, want %#v", layers, want)
	}
}

func TestWorkspaceValidateRejectsCycle(t *testing.T) {
	t.Parallel()

	workspace := fixtureWorkspace()
	workspace.Dependencies = append(workspace.Dependencies, Dependency{
		ProjectID: "database", DependsOnProjectID: "web",
	})
	if err := workspace.Validate(); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("Validate() error = %v, want ErrDependencyCycle", err)
	}
}

func TestWorkspaceValidateRejectsInvalidReferencesAndDuplicates(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*Workspace){
		"duplicate member": func(workspace *Workspace) {
			workspace.Members = append(workspace.Members, workspace.Members[0])
		},
		"missing dependency member": func(workspace *Workspace) {
			workspace.Dependencies = append(workspace.Dependencies, Dependency{ProjectID: "web", DependsOnProjectID: "missing"})
		},
		"missing profile member": func(workspace *Workspace) {
			workspace.Profiles[0].ProjectIDs = append(workspace.Profiles[0].ProjectIDs, "missing")
		},
		"missing recipe member": func(workspace *Workspace) {
			workspace.Recipes[0].ProjectID = "missing"
		},
		"health gate without timeout": func(workspace *Workspace) {
			workspace.Members[0].HealthGate = true
			workspace.Members[0].HealthTimeout = 0
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workspace := fixtureWorkspace()
			mutate(&workspace)
			if err := workspace.Validate(); !errors.Is(err, ErrInvalidWorkspace) {
				t.Fatalf("Validate() error = %v, want ErrInvalidWorkspace", err)
			}
		})
	}
}

func TestWorkspaceSelectionAddsDependencyClosureAndUsesLowMemoryLimit(t *testing.T) {
	t.Parallel()

	workspace := fixtureWorkspace()
	selected, maxParallel, profileID, err := workspace.Selection("low-memory")
	if err != nil {
		t.Fatalf("Selection() error = %v", err)
	}
	if profileID != "low-memory" || maxParallel != 1 {
		t.Fatalf("Selection() profile = %q max = %d", profileID, maxParallel)
	}
	want := map[string]struct{}{"database": {}, "api": {}, "web": {}}
	if !reflect.DeepEqual(selected, want) {
		t.Fatalf("Selection() = %#v, want %#v", selected, want)
	}
	layers, err := workspace.TopologicalLayers(selected)
	if err != nil {
		t.Fatalf("TopologicalLayers(selection) error = %v", err)
	}
	if wantLayers := [][]string{{"database"}, {"api"}, {"web"}}; !reflect.DeepEqual(layers, wantLayers) {
		t.Fatalf("TopologicalLayers(selection) = %#v, want %#v", layers, wantLayers)
	}
}

func fixtureWorkspace() Workspace {
	return Workspace{
		ID: "workspace-1", Name: "Product", DefaultFailurePolicy: FailurePolicyRollback,
		DefaultProfileID: "low-memory", CreatedAt: time.Now(), UpdatedAt: time.Now(), Revision: 1,
		Members: []Member{
			{ProjectID: "database", Role: MemberRoleDependency, Order: 0},
			{ProjectID: "cache", Role: MemberRoleDependency, Order: 1},
			{ProjectID: "api", Role: MemberRoleApplication, Order: 2, HealthGate: true, HealthTimeout: time.Minute},
			{ProjectID: "worker", Role: MemberRoleApplication, Order: 3},
			{ProjectID: "web", Role: MemberRoleApplication, Order: 4},
		},
		Dependencies: []Dependency{
			{ProjectID: "api", DependsOnProjectID: "database"},
			{ProjectID: "worker", DependsOnProjectID: "database"},
			{ProjectID: "worker", DependsOnProjectID: "cache"},
			{ProjectID: "web", DependsOnProjectID: "api"},
		},
		Profiles: []Profile{{
			ID: "low-memory", Name: "Low memory", ProjectIDs: []string{"web"}, MaxParallel: 1, LowMemory: true,
		}},
		Recipes: []Recipe{{
			ID: "open-web", Name: "Open web", Kind: RecipeOpenURL, ProjectID: "web", Target: "http://web.localhost",
		}},
	}
}
