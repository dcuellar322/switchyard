package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	application "switchyard.dev/switchyard/internal/catalog/application"
	catalog "switchyard.dev/switchyard/internal/catalog/domain"
	discovery "switchyard.dev/switchyard/internal/discovery/domain"
	manifest "switchyard.dev/switchyard/internal/manifest/domain"
)

// CatalogRepository persists project onboarding aggregates transactionally.
type CatalogRepository struct{ database *Database }

// NewCatalogRepository creates a durable catalog adapter.
func NewCatalogRepository(database *Database) *CatalogRepository {
	return &CatalogRepository{database: database}
}

// FindProposalByLocation deduplicates scans for one canonical repository root.
func (r *CatalogRepository) FindProposalByLocation(ctx context.Context, location string) (catalog.Project, discovery.Proposal, error) {
	var projectID, proposalID string
	err := r.database.connection.QueryRowContext(ctx, `SELECT p.id, mp.id FROM projects p
        JOIN manifest_proposals mp ON mp.project_id = p.id
        WHERE p.primary_location = ? ORDER BY mp.created_at DESC LIMIT 1`, location).Scan(&projectID, &proposalID)
	if errors.Is(err, sql.ErrNoRows) {
		return catalog.Project{}, discovery.Proposal{}, application.ErrNotFound
	}
	if err != nil {
		return catalog.Project{}, discovery.Proposal{}, fmt.Errorf("find project proposal by location: %w", err)
	}
	project, err := r.GetProject(ctx, projectID)
	if err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	proposal, err := r.GetProposal(ctx, proposalID)
	return project, proposal, err
}

// CreateProposal atomically stores a project, proposal, evidence, and audit event.
func (r *CatalogRepository) CreateProposal(ctx context.Context, project catalog.Project, proposal discovery.Proposal) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO projects
        (id, slug, display_name, description, trust_state, primary_location, manifest_revision, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, project.ID, project.Slug, project.DisplayName, project.Description,
		project.TrustState, project.PrimaryLocation, project.ManifestRevision, formatTime(project.CreatedAt), formatTime(project.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_locations (project_id, path, is_primary) VALUES (?, ?, 1)`, project.ID, project.PrimaryLocation); err != nil {
		return fmt.Errorf("insert project location: %w", err)
	}
	for _, tag := range project.Tags {
		if _, err = tx.ExecContext(ctx, `INSERT INTO project_tags (project_id, tag) VALUES (?, ?)`, project.ID, tag); err != nil {
			return fmt.Errorf("insert project tag: %w", err)
		}
	}
	candidate, _ := json.Marshal(proposal.Candidate)
	confidence, _ := json.Marshal(proposal.ConfidenceByField)
	unresolved, _ := json.Marshal(proposal.Unresolved)
	validation, _ := json.Marshal(proposal.Validation)
	_, err = tx.ExecContext(ctx, `INSERT INTO manifest_proposals
        (id, project_id, scanner_version, schema_version, candidate_json, confidence_json, unresolved_json, validation_json, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, proposal.ID, proposal.ProjectID, proposal.ScannerVersion,
		proposal.SchemaVersion, candidate, confidence, unresolved, validation, proposal.Status, formatTime(proposal.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert manifest proposal: %w", err)
	}
	for _, item := range proposal.Evidence {
		warnings, _ := json.Marshal(item.Warnings)
		_, err = tx.ExecContext(ctx, `INSERT INTO discovery_evidence
            (id, proposal_id, scanner, kind, source_path, start_line, end_line, confidence, data_json, warnings_json)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, proposal.ID, item.Scanner, item.Kind, item.SourcePath,
			item.Location.StartLine, item.Location.EndLine, item.Confidence, item.Data, warnings)
		if err != nil {
			return fmt.Errorf("insert discovery evidence: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events
        (event_type, actor_type, actor_id, project_id, idempotency_key, detail_json, occurred_at)
        VALUES ('manifest.proposal.created', 'system', 'catalog-service', ?, ?, '{}', ?)`, project.ID, proposal.ID, formatTime(proposal.CreatedAt)); err != nil {
		return fmt.Errorf("audit manifest proposal: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog transaction: %w", err)
	}
	return nil
}

// GetProposal rehydrates a proposal aggregate with normalized evidence.
func (r *CatalogRepository) GetProposal(ctx context.Context, id string) (discovery.Proposal, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, project_id, scanner_version, schema_version,
        candidate_json, confidence_json, unresolved_json, validation_json, status, created_at
        FROM manifest_proposals WHERE id = ?`, id)
	var proposal discovery.Proposal
	var candidate, confidence, unresolved, validation, created string
	if err := row.Scan(&proposal.ID, &proposal.ProjectID, &proposal.ScannerVersion, &proposal.SchemaVersion,
		&candidate, &confidence, &unresolved, &validation, &proposal.Status, &created); errors.Is(err, sql.ErrNoRows) {
		return discovery.Proposal{}, application.ErrNotFound
	} else if err != nil {
		return discovery.Proposal{}, fmt.Errorf("read manifest proposal: %w", err)
	}
	if err := decodeJSON(candidate, &proposal.Candidate); err != nil {
		return discovery.Proposal{}, err
	}
	if err := decodeJSON(confidence, &proposal.ConfidenceByField); err != nil {
		return discovery.Proposal{}, err
	}
	if err := decodeJSON(unresolved, &proposal.Unresolved); err != nil {
		return discovery.Proposal{}, err
	}
	if err := decodeJSON(validation, &proposal.Validation); err != nil {
		return discovery.Proposal{}, err
	}
	proposal.CreatedAt, _ = parseTime(created)
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, scanner, kind, source_path, start_line, end_line,
        confidence, data_json, warnings_json FROM discovery_evidence WHERE proposal_id = ? ORDER BY source_path, start_line, id`, id)
	if err != nil {
		return discovery.Proposal{}, fmt.Errorf("read proposal evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	proposal.Evidence = []discovery.Evidence{}
	for rows.Next() {
		var item discovery.Evidence
		var data, warnings string
		if err := rows.Scan(&item.ID, &item.Scanner, &item.Kind, &item.SourcePath, &item.Location.StartLine,
			&item.Location.EndLine, &item.Confidence, &data, &warnings); err != nil {
			return discovery.Proposal{}, err
		}
		item.Data = json.RawMessage(data)
		if err := decodeJSON(warnings, &item.Warnings); err != nil {
			return discovery.Proposal{}, err
		}
		proposal.Evidence = append(proposal.Evidence, item)
	}
	return proposal, rows.Err()
}

// AcceptProposal atomically trusts a proposal and appends a manifest revision.
func (r *CatalogRepository) AcceptProposal(ctx context.Context, id string, at time.Time) (catalog.Project, discovery.Proposal, error) {
	proposal, err := r.GetProposal(ctx, id)
	if err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	if proposal.Status != discovery.StatusProposed {
		return catalog.Project{}, discovery.Proposal{}, application.ErrAlreadyReviewed
	}
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE manifest_proposals SET status = 'accepted' WHERE id = ? AND status = 'proposed'`, id)
	if err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return catalog.Project{}, discovery.Proposal{}, application.ErrAlreadyReviewed
	}
	if _, err = tx.ExecContext(ctx, `UPDATE manifest_proposals SET status = 'superseded' WHERE project_id = ? AND id <> ? AND status = 'accepted'`, proposal.ProjectID, id); err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT manifest_revision + 1 FROM projects WHERE id = ?`, proposal.ProjectID).Scan(&revision); err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	candidate, _ := json.Marshal(proposal.Candidate)
	if _, err = tx.ExecContext(ctx, `INSERT INTO manifest_snapshots (project_id, revision, proposal_id, manifest_json, created_at) VALUES (?, ?, ?, ?, ?)`, proposal.ProjectID, revision, id, candidate, formatTime(at)); err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET trust_state = 'trusted', manifest_revision = ?, updated_at = ? WHERE id = ?`, revision, formatTime(at), proposal.ProjectID); err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events
        (event_type, actor_type, actor_id, project_id, idempotency_key, detail_json, occurred_at)
        VALUES ('manifest.proposal.accepted', 'system', 'catalog-service', ?, ?, '{}', ?)`, proposal.ProjectID, proposal.ID, formatTime(at)); err != nil {
		return catalog.Project{}, discovery.Proposal{}, fmt.Errorf("audit accepted manifest: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return catalog.Project{}, discovery.Proposal{}, err
	}
	proposal.Status = discovery.StatusAccepted
	project, err := r.GetProject(ctx, proposal.ProjectID)
	return project, proposal, err
}

// GetProject returns one durable project with its tags.
func (r *CatalogRepository) GetProject(ctx context.Context, id string) (catalog.Project, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, slug, display_name, description, trust_state,
        primary_location, manifest_revision, created_at, updated_at FROM projects WHERE id = ?`, id)
	project, err := scanProject(row)
	if err != nil {
		return catalog.Project{}, err
	}
	project.Tags, err = r.projectTags(ctx, project.ID)
	return project, err
}

// ListProjects returns all durable projects with their tags.
func (r *CatalogRepository) ListProjects(ctx context.Context) ([]catalog.Project, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, slug, display_name, description, trust_state,
        primary_location, manifest_revision, created_at, updated_at FROM projects ORDER BY display_name, id`)
	if err != nil {
		return nil, err
	}
	result := []catalog.Project{}
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Tags, err = r.projectTags(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *CatalogRepository) projectTags(ctx context.Context, projectID string) ([]string, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT tag FROM project_tags WHERE project_id = ? ORDER BY tag`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// AcceptedManifest returns the latest immutable trusted manifest revision.
func (r *CatalogRepository) AcceptedManifest(ctx context.Context, projectID string) (manifest.Manifest, error) {
	var value string
	err := r.database.connection.QueryRowContext(ctx, `SELECT manifest_json FROM manifest_snapshots WHERE project_id = ? ORDER BY revision DESC LIMIT 1`, projectID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return manifest.Manifest{}, application.ErrNotFound
	}
	if err != nil {
		return manifest.Manifest{}, err
	}
	var result manifest.Manifest
	return result, decodeJSON(value, &result)
}

type rowScanner interface{ Scan(...any) error }

func scanProject(row rowScanner) (catalog.Project, error) {
	var project catalog.Project
	var created, updated string
	err := row.Scan(&project.ID, &project.Slug, &project.DisplayName, &project.Description, &project.TrustState,
		&project.PrimaryLocation, &project.ManifestRevision, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return catalog.Project{}, application.ErrNotFound
	}
	if err != nil {
		return catalog.Project{}, err
	}
	project.CreatedAt, _ = parseTime(created)
	project.UpdatedAt, _ = parseTime(updated)
	project.Tags = []string{}
	return project, nil
}
func decodeJSON(value string, target any) error {
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("decode catalog JSON: %w", err)
	}
	return nil
}
