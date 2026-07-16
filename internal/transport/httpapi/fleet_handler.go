package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	"switchyard.dev/switchyard/internal/fleet/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) ListMachines(w http.ResponseWriter, r *http.Request) {
	machines, err := h.fleet.List(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, machines)
}

func (h *handler) CreateMachine(w http.ResponseWriter, r *http.Request) {
	var request generated.MachineRegistrationRequest
	if err := decodeFleetBody(w, r, &request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "MACHINE_REQUEST_INVALID", "Machine registration invalid", "Provide a bounded certificate-pinned HTTPS endpoint and absolute credential file paths.")
		return
	}
	machine, err := h.fleet.Register(r.Context(), fleetApplication.RegisterRequest{
		Name: request.Name, Endpoint: request.Endpoint, CertificateFingerprint: request.CertificateFingerprint,
		Credentials: domain.CredentialReferences{
			CACertificate: request.CaCertificatePath, ClientCertificate: request.ClientCertificatePath, ClientKey: request.ClientKeyPath,
		},
		GrantedCapabilities: fleetCapabilities(request.GrantedCapabilities), ConfirmRisk: request.ConfirmRisk,
	}, requestFleetActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, machine)
}

func (h *handler) GetMachine(w http.ResponseWriter, r *http.Request, machineID generated.MachineId) {
	machine, err := h.fleet.Get(r.Context(), machineID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

func (h *handler) DeleteMachine(w http.ResponseWriter, r *http.Request, machineID generated.MachineId, params generated.DeleteMachineParams) {
	if err := h.fleet.Remove(r.Context(), machineID, params.ConfirmRisk, requestFleetActor(r)); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) UpdateMachineAccess(w http.ResponseWriter, r *http.Request, machineID generated.MachineId) {
	var request generated.MachineAccessRequest
	if err := decodeFleetBody(w, r, &request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "MACHINE_ACCESS_INVALID", "Machine access invalid", "Provide the complete reviewed capability grant set.")
		return
	}
	machine, err := h.fleet.ConfigureAccess(
		r.Context(), machineID, request.Enabled, fleetCapabilities(request.GrantedCapabilities), request.ConfirmRisk, requestFleetActor(r),
	)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

func (h *handler) ProbeMachine(w http.ResponseWriter, r *http.Request, machineID generated.MachineId) {
	machine, err := h.fleet.Probe(r.Context(), machineID, requestFleetActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

func (h *handler) GetMachineSnapshot(w http.ResponseWriter, r *http.Request, machineID generated.MachineId) {
	snapshot, err := h.fleet.Snapshot(r.Context(), machineID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *handler) CreateMachineOperation(w http.ResponseWriter, r *http.Request, machineID generated.MachineId) {
	var request generated.RemoteOperationRequest
	if err := decodeFleetBody(w, r, &request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REMOTE_OPERATION_INVALID", "Remote operation invalid", "Provide one supported typed lifecycle action and explicit confirmation.")
		return
	}
	environmentID := ""
	if request.EnvironmentId != nil {
		environmentID = *request.EnvironmentId
	}
	receipt, err := h.fleet.Operate(r.Context(), machineID, domain.OperationRequest{
		RequestID: request.RequestId, ProjectID: request.ProjectId, EnvironmentID: environmentID,
		Action: domain.OperationAction(request.Action), ConfirmRisk: request.ConfirmRisk,
	}, requestFleetActor(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func decodeFleetBody(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains multiple JSON values")
	}
	return nil
}

func fleetCapabilities(values []generated.FleetCapability) []domain.Capability {
	result := make([]domain.Capability, len(values))
	for index, capability := range values {
		result[index] = domain.Capability(capability)
	}
	return result
}

func requestFleetActor(r *http.Request) fleetApplication.Actor {
	actorType, actorID := RequestActor(r.Context())
	return fleetApplication.Actor{Type: actorType, ID: actorID}
}
