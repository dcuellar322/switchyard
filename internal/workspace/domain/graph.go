package domain

import (
	"cmp"
	"fmt"
	"slices"
)

// Selection resolves a profile to a dependency-closed project set and concurrency cap.
func (w Workspace) Selection(profileID string) (map[string]struct{}, int, string, error) {
	if profileID == "" {
		profileID = w.DefaultProfileID
	}
	selected := make(map[string]struct{}, len(w.Members))
	maxParallel := 4
	if profileID == "" {
		for _, member := range w.Members {
			selected[member.ProjectID] = struct{}{}
		}
		return selected, maxParallel, "", nil
	}
	profile, ok := w.Profile(profileID)
	if !ok {
		return nil, 0, "", fmt.Errorf("%w: %s", ErrUnknownProfile, profileID)
	}
	for _, projectID := range profile.ProjectIDs {
		selected[projectID] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, edge := range w.Dependencies {
			if _, included := selected[edge.ProjectID]; !included {
				continue
			}
			if _, included := selected[edge.DependsOnProjectID]; !included {
				selected[edge.DependsOnProjectID] = struct{}{}
				changed = true
			}
		}
	}
	return selected, profile.MaxParallel, profile.ID, nil
}

// TopologicalLayers returns dependency-first layers in deterministic member order.
// A nil selected set includes every workspace member.
func (w Workspace) TopologicalLayers(selected map[string]struct{}) ([][]string, error) {
	if selected == nil {
		selected = make(map[string]struct{}, len(w.Members))
		for _, member := range w.Members {
			selected[member.ProjectID] = struct{}{}
		}
	}
	memberOrder := make(map[string]int, len(w.Members))
	indegree := make(map[string]int, len(selected))
	dependents := make(map[string][]string, len(selected))
	for _, member := range w.Members {
		memberOrder[member.ProjectID] = member.Order
		if _, ok := selected[member.ProjectID]; ok {
			indegree[member.ProjectID] = 0
		}
	}
	for _, edge := range w.Dependencies {
		_, sourceIncluded := selected[edge.ProjectID]
		_, targetIncluded := selected[edge.DependsOnProjectID]
		if !sourceIncluded || !targetIncluded {
			continue
		}
		indegree[edge.ProjectID]++
		dependents[edge.DependsOnProjectID] = append(dependents[edge.DependsOnProjectID], edge.ProjectID)
	}
	less := func(left, right string) int {
		if order := cmp.Compare(memberOrder[left], memberOrder[right]); order != 0 {
			return order
		}
		return cmp.Compare(left, right)
	}
	remaining := len(indegree)
	layers := make([][]string, 0)
	for remaining > 0 {
		layer := make([]string, 0)
		for projectID, count := range indegree {
			if count == 0 {
				layer = append(layer, projectID)
			}
		}
		if len(layer) == 0 {
			return nil, ErrDependencyCycle
		}
		slices.SortFunc(layer, less)
		layers = append(layers, layer)
		for _, projectID := range layer {
			delete(indegree, projectID)
			remaining--
			for _, dependent := range dependents[projectID] {
				indegree[dependent]--
			}
		}
	}
	return layers, nil
}

// DependenciesOf returns selected direct dependencies for one project.
func (w Workspace) DependenciesOf(projectID string, selected map[string]struct{}) []string {
	result := make([]string, 0)
	for _, edge := range w.Dependencies {
		if edge.ProjectID != projectID {
			continue
		}
		if _, ok := selected[edge.DependsOnProjectID]; ok {
			result = append(result, edge.DependsOnProjectID)
		}
	}
	slices.Sort(result)
	return result
}

// DependentsOf returns selected direct dependents for one project.
func (w Workspace) DependentsOf(projectID string, selected map[string]struct{}) []string {
	result := make([]string, 0)
	for _, edge := range w.Dependencies {
		if edge.DependsOnProjectID != projectID {
			continue
		}
		if _, ok := selected[edge.ProjectID]; ok {
			result = append(result, edge.ProjectID)
		}
	}
	slices.Sort(result)
	return result
}

// Member returns one workspace member by project ID.
func (w Workspace) Member(projectID string) (Member, bool) {
	for _, member := range w.Members {
		if member.ProjectID == projectID {
			return member, true
		}
	}
	return Member{}, false
}
