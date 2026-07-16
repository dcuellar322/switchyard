package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	teamApplication "switchyard.dev/switchyard/internal/team/application"
	"switchyard.dev/switchyard/internal/team/domain"
)

// TeamRepository persists reviewed publisher trust and verified bundles.
type TeamRepository struct{ database *Database }

func NewTeamRepository(database *Database) *TeamRepository {
	return &TeamRepository{database: database}
}

func (r *TeamRepository) TrustPublisher(ctx context.Context, publisher domain.Publisher) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO team_publishers(id, name, public_key, trusted_at)
        VALUES (?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, public_key=excluded.public_key,
        trusted_at=excluded.trusted_at`, publisher.ID, publisher.Name, publisher.PublicKey, formatTime(publisher.TrustedAt))
	if err != nil {
		return fmt.Errorf("trust team publisher: %w", err)
	}
	return nil
}

func (r *TeamRepository) ListPublishers(ctx context.Context) ([]domain.Publisher, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, name, public_key, trusted_at FROM team_publishers ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []domain.Publisher{}
	for rows.Next() {
		publisher, err := scanPublisher(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, publisher)
	}
	return result, rows.Err()
}

func (r *TeamRepository) GetPublisher(ctx context.Context, id string) (domain.Publisher, error) {
	publisher, err := scanPublisher(r.database.connection.QueryRowContext(ctx, `SELECT id, name, public_key, trusted_at FROM team_publishers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Publisher{}, teamApplication.ErrNotFound
	}
	return publisher, err
}

func (r *TeamRepository) InstallBundle(ctx context.Context, bundle domain.Bundle) error {
	if bundle.InstalledAt == nil {
		return errors.New("verified bundle requires installation time")
	}
	metadata, err := json.Marshal(bundle.Metadata)
	if err != nil {
		return err
	}
	signature, err := json.Marshal(bundle.Signature)
	if err != nil {
		return err
	}
	_, err = r.database.connection.ExecContext(ctx, `INSERT INTO team_bundles
        (id, schema_version, kind, metadata_json, payload_json, signature_json, publisher_id, installed_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET
        schema_version=excluded.schema_version, kind=excluded.kind, metadata_json=excluded.metadata_json,
        payload_json=excluded.payload_json, signature_json=excluded.signature_json,
        publisher_id=excluded.publisher_id, installed_at=excluded.installed_at`, bundle.Metadata.ID,
		bundle.SchemaVersion, bundle.Kind, metadata, bundle.Payload, signature, bundle.Metadata.PublisherID, formatTime(*bundle.InstalledAt))
	if err != nil {
		return fmt.Errorf("install team bundle: %w", err)
	}
	return nil
}

func (r *TeamRepository) ListBundles(ctx context.Context, kind domain.BundleKind) ([]domain.Bundle, error) {
	query := `SELECT id, schema_version, kind, metadata_json, payload_json, signature_json, installed_at FROM team_bundles`
	arguments := []any{}
	if kind != "" {
		query += ` WHERE kind = ?`
		arguments = append(arguments, kind)
	}
	query += ` ORDER BY kind, installed_at DESC, id`
	rows, err := r.database.connection.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []domain.Bundle{}
	for rows.Next() {
		bundle, err := scanBundle(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, bundle)
	}
	return result, rows.Err()
}

func (r *TeamRepository) GetBundle(ctx context.Context, id string) (domain.Bundle, error) {
	bundle, err := scanBundle(r.database.connection.QueryRowContext(ctx, `SELECT id, schema_version, kind,
        metadata_json, payload_json, signature_json, installed_at FROM team_bundles WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Bundle{}, teamApplication.ErrNotFound
	}
	return bundle, err
}

func (r *TeamRepository) ApplySync(ctx context.Context, document domain.SyncDocument) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, publisher := range document.Publishers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO team_publishers(id, name, public_key, trusted_at)
            VALUES (?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, public_key=excluded.public_key,
            trusted_at=excluded.trusted_at`, publisher.ID, publisher.Name, publisher.PublicKey, formatTime(publisher.TrustedAt)); err != nil {
			return fmt.Errorf("sync team publisher: %w", err)
		}
	}
	for _, bundle := range document.Bundles {
		if bundle.InstalledAt == nil {
			return errors.New("sync bundle requires installation time")
		}
		metadata, _ := json.Marshal(bundle.Metadata)
		signature, _ := json.Marshal(bundle.Signature)
		if _, err := tx.ExecContext(ctx, `INSERT INTO team_bundles
            (id, schema_version, kind, metadata_json, payload_json, signature_json, publisher_id, installed_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET
            schema_version=excluded.schema_version, kind=excluded.kind, metadata_json=excluded.metadata_json,
            payload_json=excluded.payload_json, signature_json=excluded.signature_json,
            publisher_id=excluded.publisher_id, installed_at=excluded.installed_at`, bundle.Metadata.ID,
			bundle.SchemaVersion, bundle.Kind, metadata, bundle.Payload, signature, bundle.Metadata.PublisherID, formatTime(*bundle.InstalledAt)); err != nil {
			return fmt.Errorf("sync team bundle: %w", err)
		}
	}
	return tx.Commit()
}

func (r *TeamRepository) RecordAudit(ctx context.Context, event domain.AuditEvent) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO team_audit_events
        (event_type, actor_type, actor_id, subject_id, detail, occurred_at) VALUES (?, ?, ?, ?, ?, ?)`,
		event.Type, event.ActorType, event.ActorID, event.SubjectID, event.Detail, formatTime(event.OccurredAt))
	if err != nil {
		return fmt.Errorf("record team audit: %w", err)
	}
	return nil
}

type teamScanner interface{ Scan(...any) error }

func scanPublisher(row teamScanner) (domain.Publisher, error) {
	var publisher domain.Publisher
	var trusted string
	if err := row.Scan(&publisher.ID, &publisher.Name, &publisher.PublicKey, &trusted); err != nil {
		return domain.Publisher{}, err
	}
	var err error
	publisher.TrustedAt, err = parseTime(trusted)
	return publisher, err
}

func scanBundle(row teamScanner) (domain.Bundle, error) {
	var bundle domain.Bundle
	var ignoredID, metadata, payload, signature, installed string
	if err := row.Scan(&ignoredID, &bundle.SchemaVersion, &bundle.Kind, &metadata, &payload, &signature, &installed); err != nil {
		return domain.Bundle{}, err
	}
	if err := json.Unmarshal([]byte(metadata), &bundle.Metadata); err != nil {
		return domain.Bundle{}, err
	}
	if err := json.Unmarshal([]byte(signature), &bundle.Signature); err != nil {
		return domain.Bundle{}, err
	}
	bundle.Payload = json.RawMessage(payload)
	installedAt, err := parseTime(installed)
	if err != nil {
		return domain.Bundle{}, err
	}
	bundle.InstalledAt = &installedAt
	return bundle, nil
}
