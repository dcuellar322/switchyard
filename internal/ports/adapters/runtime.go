// Package adapters observes runtime and operating-system port bindings.
package adapters

import (
	"context"
	"errors"
	"time"

	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/ports/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
)

type projectLister interface {
	ListProjects(context.Context) ([]catalogDomain.Project, error)
}

type runtimeObserver interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
}

// RuntimeBindings identifies Docker/process published ports already attributed to a project.
type RuntimeBindings struct {
	projects projectLister
	runtime  runtimeObserver
	now      func() time.Time
}

// NewRuntimeBindings creates an adapter over driver-neutral runtime observations.
func NewRuntimeBindings(projects projectLister, runtime runtimeObserver) *RuntimeBindings {
	return &RuntimeBindings{projects: projects, runtime: runtime, now: time.Now}
}

// Facts returns currently published ports attributed to trusted projects.
func (s *RuntimeBindings) Facts(ctx context.Context) ([]domain.Fact, error) {
	projects, err := s.projects.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	var facts []domain.Fact
	var observationErrors []error
	for _, project := range projects {
		if project.TrustState != catalogDomain.TrustTrusted {
			continue
		}
		observation, observeErr := s.runtime.Inspect(ctx, project.ID)
		if observeErr != nil {
			observationErrors = append(observationErrors, observeErr)
			continue
		}
		for _, service := range observation.Services {
			for _, port := range service.Ports {
				if port.HostPort == 0 {
					continue
				}
				host := port.HostIP
				if host == "" {
					host = "0.0.0.0"
				}
				facts = append(facts, domain.Fact{
					ID: runtimeFactID(project.ID, service.ID, host, port), Kind: domain.KindBinding,
					ProjectID: project.ID, ProjectName: project.DisplayName, ServiceID: service.ID,
					Host: host, Port: port.HostPort, Target: port.ContainerPort, Protocol: port.Protocol,
					Source: string(observation.Driver), Evidence: "live runtime published port", ObservedAt: s.now().UTC(),
				})
			}
		}
	}
	if len(facts) == 0 && len(observationErrors) > 0 {
		return nil, errors.Join(observationErrors...)
	}
	return facts, nil
}

func runtimeFactID(projectID, serviceID, host string, port runtimeDomain.PublishedPort) string {
	return stableID("runtime", projectID, serviceID, host, port.Protocol, port.HostPort)
}
