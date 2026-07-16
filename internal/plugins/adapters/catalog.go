package adapters

import (
	"context"

	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
)

// ProjectLookup is implemented with a function at composition time so this
// adapter exposes only the fields plugins are allowed to receive.
type ProjectLookup struct {
	lookup func(context.Context, string) (pluginsApplication.Project, error)
}

// NewProjectLookup creates the bounded catalog projection adapter.
func NewProjectLookup(lookup func(context.Context, string) (pluginsApplication.Project, error)) ProjectLookup {
	return ProjectLookup{lookup: lookup}
}

// Project returns only fields explicitly approved for the plugin boundary.
func (p ProjectLookup) Project(ctx context.Context, id string) (pluginsApplication.Project, error) {
	return p.lookup(ctx, id)
}
