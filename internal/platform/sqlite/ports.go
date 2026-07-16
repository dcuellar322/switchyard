package sqlite

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/ports/domain"
)

// PortReservationRepository persists automatically reconciled manifest leases.
type PortReservationRepository struct{ database *Database }

// NewPortReservationRepository creates the manifest lease store.
func NewPortReservationRepository(database *Database) *PortReservationRepository {
	return &PortReservationRepository{database: database}
}

// Reconcile atomically aligns manifest-backed reservations with accepted declarations.
func (r *PortReservationRepository) Reconcile(ctx context.Context, declarations []domain.Fact, now time.Time) ([]domain.Fact, error) {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	wanted := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		id := "reservation_" + declaration.ID
		wanted[id] = struct{}{}
		_, err = tx.ExecContext(ctx, `INSERT INTO port_reservations
            (id, project_id, project_name, service_id, port_id, host, port, target, protocol, source, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'manifest', ?, ?)
            ON CONFLICT(project_id, port_id, source) DO UPDATE SET
              id=excluded.id, project_name=excluded.project_name, service_id=excluded.service_id,
              host=excluded.host, port=excluded.port, target=excluded.target,
              protocol=excluded.protocol, updated_at=excluded.updated_at`,
			id, declaration.ProjectID, declaration.ProjectName, declaration.ServiceID, declaration.PortID,
			declaration.Host, declaration.Port, declaration.Target, declaration.Protocol, formatTime(now), formatTime(now))
		if err != nil {
			return nil, fmt.Errorf("upsert port reservation: %w", err)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM port_reservations WHERE source = 'manifest'`)
	if err != nil {
		return nil, err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if _, exists := wanted[id]; !exists {
			stale = append(stale, id)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM port_reservations WHERE id = ?`, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.list(ctx)
}

func (r *PortReservationRepository) list(ctx context.Context) ([]domain.Fact, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, project_id, project_name, service_id, port_id,
        host, port, target, protocol, source, updated_at FROM port_reservations ORDER BY port, project_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var facts []domain.Fact
	for rows.Next() {
		var item domain.Fact
		var observed string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectName, &item.ServiceID, &item.PortID,
			&item.Host, &item.Port, &item.Target, &item.Protocol, &item.Source, &observed); err != nil {
			return nil, err
		}
		item.Kind = domain.KindReservation
		item.Evidence = "Switchyard lease reconciled from accepted manifest"
		item.ObservedAt, err = parseTime(observed)
		if err != nil {
			return nil, err
		}
		facts = append(facts, item)
	}
	return facts, rows.Err()
}
