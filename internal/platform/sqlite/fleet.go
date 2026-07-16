package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	"switchyard.dev/switchyard/internal/fleet/domain"
)

// FleetRepository persists explicitly configured peers and redacted snapshots.
type FleetRepository struct{ database *Database }

// NewFleetRepository creates the local fleet persistence adapter.
func NewFleetRepository(database *Database) *FleetRepository {
	return &FleetRepository{database: database}
}

// Create records certificate file references and explicit capability grants.
func (r *FleetRepository) Create(ctx context.Context, machine domain.Machine) error {
	capabilities, _ := json.Marshal(nonNilCapabilities(machine.Capabilities))
	grants, _ := json.Marshal(nonNilCapabilities(machine.GrantedCapabilities))
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO fleet_machines
        (id, name, endpoint, certificate_fingerprint, ca_certificate_path, client_certificate_path, client_key_path,
         enabled, capabilities_json, grants_json, state, peer_id, peer_version, os, architecture, last_error,
         last_seen_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		machine.ID, machine.Name, machine.Endpoint, machine.CertificateFingerprint,
		machine.Credentials.CACertificate, machine.Credentials.ClientCertificate, machine.Credentials.ClientKey,
		boolInteger(machine.Enabled), capabilities, grants, machine.State, machine.PeerID, machine.PeerVersion,
		machine.OS, machine.Architecture, machine.LastError, optionalTime(machine.LastSeenAt),
		formatTime(machine.CreatedAt), formatTime(machine.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create fleet machine: %w", err)
	}
	return nil
}

// List returns peers in stable enabled, state, and display-name order.
func (r *FleetRepository) List(ctx context.Context) ([]domain.Machine, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, name, endpoint, certificate_fingerprint,
        ca_certificate_path, client_certificate_path, client_key_path, enabled, capabilities_json, grants_json,
        state, peer_id, peer_version, os, architecture, last_error, last_seen_at, created_at, updated_at
        FROM fleet_machines ORDER BY enabled DESC, state, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("list fleet machines: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []domain.Machine{}
	for rows.Next() {
		machine, err := scanMachine(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, machine)
	}
	return result, rows.Err()
}

// Get returns one peer including local-only credential references.
func (r *FleetRepository) Get(ctx context.Context, id string) (domain.Machine, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, name, endpoint, certificate_fingerprint,
        ca_certificate_path, client_certificate_path, client_key_path, enabled, capabilities_json, grants_json,
        state, peer_id, peer_version, os, architecture, last_error, last_seen_at, created_at, updated_at
        FROM fleet_machines WHERE id = ?`, id)
	machine, err := scanMachine(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Machine{}, fleetApplication.ErrNotFound
	}
	return machine, err
}

// UpdateAccess changes the complete reviewed grant set and enabled state.
func (r *FleetRepository) UpdateAccess(ctx context.Context, id string, enabled bool, grants []domain.Capability, now time.Time) error {
	encoded, _ := json.Marshal(nonNilCapabilities(grants))
	result, err := r.database.connection.ExecContext(ctx, `UPDATE fleet_machines
        SET enabled = ?, grants_json = ?, state = CASE WHEN ? = 1 THEN
          CASE WHEN state = 'disabled' THEN 'pending' ELSE state END ELSE 'disabled' END,
          updated_at = ? WHERE id = ?`, boolInteger(enabled), encoded, boolInteger(enabled), formatTime(now), id)
	if err != nil {
		return fmt.Errorf("update fleet access: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return fleetApplication.ErrNotFound
	}
	return nil
}

// RecordObservation atomically updates peer identity and, when present, the
// latest bounded inventory snapshot.
func (r *FleetRepository) RecordObservation(ctx context.Context, id string, snapshot domain.Snapshot, state domain.MachineState, lastError string, now time.Time) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	identity := snapshot.Identity
	capabilities, _ := json.Marshal(nonNilCapabilities(identity.Capabilities))
	var lastSeen any
	if identity.MachineID != "" {
		lastSeen = formatTime(now)
	}
	result, err := tx.ExecContext(ctx, `UPDATE fleet_machines SET capabilities_json = CASE WHEN ? = '' THEN capabilities_json ELSE ? END,
        state = ?, peer_id = CASE WHEN ? = '' THEN peer_id ELSE ? END,
        peer_version = CASE WHEN ? = '' THEN peer_version ELSE ? END,
        os = CASE WHEN ? = '' THEN os ELSE ? END, architecture = CASE WHEN ? = '' THEN architecture ELSE ? END,
        last_error = ?, last_seen_at = COALESCE(?, last_seen_at), updated_at = ? WHERE id = ?`,
		identity.MachineID, capabilities, state, identity.MachineID, identity.MachineID,
		identity.Version, identity.Version, identity.OS, identity.OS, identity.Architecture, identity.Architecture,
		lastError, lastSeen, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("record fleet observation: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return fleetApplication.ErrNotFound
	}
	if identity.MachineID != "" {
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO fleet_snapshots(machine_id, snapshot_json, observed_at)
            VALUES (?, ?, ?) ON CONFLICT(machine_id) DO UPDATE SET
            snapshot_json = excluded.snapshot_json, observed_at = excluded.observed_at`, id, encoded, formatTime(snapshot.ObservedAt)); err != nil {
			return fmt.Errorf("record fleet snapshot: %w", err)
		}
	}
	return tx.Commit()
}

// Delete removes a peer. Audit rows intentionally retain its stable ID.
func (r *FleetRepository) Delete(ctx context.Context, id string) error {
	result, err := r.database.connection.ExecContext(ctx, `DELETE FROM fleet_machines WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return fleetApplication.ErrNotFound
	}
	return nil
}

// RecordAudit appends a non-secret fleet authorization event.
func (r *FleetRepository) RecordAudit(ctx context.Context, event domain.AuditEvent) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO fleet_audit_events
        (machine_id, event_type, actor_type, actor_id, request_id, detail, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.MachineID, event.Type, event.ActorType, event.ActorID, event.RequestID, event.Detail, formatTime(event.OccurredAt))
	if err != nil {
		return fmt.Errorf("record fleet audit: %w", err)
	}
	return nil
}

type fleetMachineScanner interface{ Scan(...any) error }

func scanMachine(row fleetMachineScanner) (domain.Machine, error) {
	var machine domain.Machine
	var enabled int
	var capabilities, grants, created, updated string
	var lastSeen sql.NullString
	err := row.Scan(&machine.ID, &machine.Name, &machine.Endpoint, &machine.CertificateFingerprint,
		&machine.Credentials.CACertificate, &machine.Credentials.ClientCertificate, &machine.Credentials.ClientKey,
		&enabled, &capabilities, &grants, &machine.State, &machine.PeerID, &machine.PeerVersion, &machine.OS,
		&machine.Architecture, &machine.LastError, &lastSeen, &created, &updated)
	if err != nil {
		return domain.Machine{}, err
	}
	if err := json.Unmarshal([]byte(capabilities), &machine.Capabilities); err != nil {
		return domain.Machine{}, fmt.Errorf("decode fleet capabilities: %w", err)
	}
	if err := json.Unmarshal([]byte(grants), &machine.GrantedCapabilities); err != nil {
		return domain.Machine{}, fmt.Errorf("decode fleet grants: %w", err)
	}
	machine.Enabled = enabled == 1
	machine.CredentialConfigured = machine.Credentials.Complete()
	if lastSeen.Valid {
		value, err := parseTime(lastSeen.String)
		if err != nil {
			return domain.Machine{}, err
		}
		machine.LastSeenAt = &value
	}
	if machine.CreatedAt, err = parseTime(created); err != nil {
		return domain.Machine{}, err
	}
	if machine.UpdatedAt, err = parseTime(updated); err != nil {
		return domain.Machine{}, err
	}
	return machine, nil
}

func nonNilCapabilities(values []domain.Capability) []domain.Capability {
	if values == nil {
		return []domain.Capability{}
	}
	return values
}

func optionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
