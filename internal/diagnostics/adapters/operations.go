package adapters

import (
	"context"

	diagnosticsApplication "switchyard.dev/switchyard/internal/diagnostics/application"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
)

type operationSubmitter interface {
	Submit(context.Context, operationsApplication.SubmitRequest) (operationsDomain.Operation, error)
}

// ActionSubmitter routes automation through the existing operation and action permission kernels.
type ActionSubmitter struct{ operations operationSubmitter }

// NewActionSubmitter creates an automation operation adapter.
func NewActionSubmitter(operations operationSubmitter) *ActionSubmitter {
	return &ActionSubmitter{operations: operations}
}

// SubmitAction emits only the existing action.run operation contract.
func (s *ActionSubmitter) SubmitAction(ctx context.Context, projectID, actionID, recipeID, diagnosisID string) (string, error) {
	operation, err := s.operations.Submit(ctx, operationsApplication.SubmitRequest{
		ProjectID: projectID, Kind: "action.run", IdempotencyKey: "automation:" + recipeID + ":" + diagnosisID,
		Input: diagnosticsApplication.ActionOperationInput(actionID, recipeID), ActorType: "automation", ActorID: recipeID,
	})
	return operation.ID, err
}
