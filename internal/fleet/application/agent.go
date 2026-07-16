package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/fleet/domain"
)

type LocalInventory interface {
	Inventory(context.Context) ([]domain.Project, []domain.Environment, error)
}

type LocalOperator interface {
	SubmitRemote(context.Context, domain.OperationRequest, string) (domain.OperationReceipt, error)
}

// ControllerGrant is one certificate-pinned controller and its independent
// application-level capabilities. TLS allowlisting alone is not authorization.
type ControllerGrant struct {
	Fingerprint  string
	Capabilities []domain.Capability
}

type AgentService struct {
	identity    domain.Identity
	inventory   LocalInventory
	operator    LocalOperator
	controllers map[string][]domain.Capability
	now         func() time.Time
}

func NewAgentService(identity domain.Identity, inventory LocalInventory, operator LocalOperator, controllers []ControllerGrant) (*AgentService, error) {
	if err := identity.Validate(); err != nil || inventory == nil || operator == nil || len(controllers) == 0 {
		return nil, errors.New("remote agent dependencies and identity are required")
	}
	grants := make(map[string][]domain.Capability, len(controllers))
	for _, controller := range controllers {
		fingerprint, err := normalizeFingerprint(controller.Fingerprint)
		if err != nil {
			return nil, err
		}
		capabilities, err := normalizeCapabilities(controller.Capabilities)
		if err != nil || len(capabilities) == 0 {
			return nil, errors.New("remote controller requires explicit capabilities")
		}
		grants[fingerprint] = capabilities
	}
	return &AgentService{identity: identity, inventory: inventory, operator: operator, controllers: grants, now: time.Now}, nil
}

func (s *AgentService) Identity(controller string) (domain.Identity, error) {
	if !s.authorized(controller, domain.CapabilityInventoryRead) {
		return domain.Identity{}, ErrPermissionDenied
	}
	return s.identity, nil
}

func (s *AgentService) Snapshot(ctx context.Context, controller string) (domain.Snapshot, error) {
	if !s.authorized(controller, domain.CapabilityInventoryRead) {
		return domain.Snapshot{}, ErrPermissionDenied
	}
	projects, environments, err := s.inventory.Inventory(ctx)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{Identity: s.identity, Projects: projects, Environments: environments, ObservedAt: s.now().UTC()}, nil
}

func (s *AgentService) Operate(ctx context.Context, controller string, request domain.OperationRequest) (domain.OperationReceipt, error) {
	capability := domain.CapabilityProjectOperate
	if request.EnvironmentID != "" {
		capability = domain.CapabilityEnvironmentManage
	}
	if !s.authorized(controller, capability) {
		return domain.OperationReceipt{}, ErrPermissionDenied
	}
	if !request.ConfirmRisk {
		return domain.OperationReceipt{}, ErrConfirmationNeeded
	}
	if request.RequestID == "" || request.ProjectID == "" || !request.Action.Valid() {
		return domain.OperationReceipt{}, errors.New("remote operation request is invalid")
	}
	return s.operator.SubmitRemote(ctx, request, strings.ToLower(controller))
}

func (s *AgentService) authorized(controller string, capability domain.Capability) bool {
	fingerprint := strings.ToLower(strings.ReplaceAll(controller, ":", ""))
	return slices.Contains(s.controllers[fingerprint], capability)
}

func (s *AgentService) ControllerFingerprints() []string {
	result := make([]string, 0, len(s.controllers))
	for fingerprint := range s.controllers {
		result = append(result, fingerprint)
	}
	slices.Sort(result)
	return result
}
