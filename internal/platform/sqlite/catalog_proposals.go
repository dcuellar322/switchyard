package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	application "switchyard.dev/switchyard/internal/catalog/application"
	catalog "switchyard.dev/switchyard/internal/catalog/domain"
	discovery "switchyard.dev/switchyard/internal/discovery/domain"
)

// ReplacePendingProposal atomically refreshes an untrusted project and supersedes its prior proposal.
func (r *CatalogRepository) ReplacePendingProposal(
	ctx context.Context,
	sourceID string,
	project catalog.Project,
	proposal discovery.Proposal,
	actor application.MutationActor,
) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin proposal refresh: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE projects SET slug = ?, display_name = ?, description = ?, updated_at = ?
		WHERE id = ? AND trust_state = 'pending'`, project.Slug, project.DisplayName, project.Description, formatTime(project.UpdatedAt), project.ID)
	if err != nil {
		return fmt.Errorf("refresh pending project: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return application.ErrAlreadyReviewed
	}
	result, err = tx.ExecContext(ctx, `UPDATE manifest_proposals SET status = 'superseded'
		WHERE id = ? AND project_id = ? AND status = 'proposed'`, sourceID, project.ID)
	if err != nil {
		return fmt.Errorf("supersede refreshed proposal: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return application.ErrAlreadyReviewed
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM project_tags WHERE project_id = ?`, project.ID); err != nil {
		return fmt.Errorf("clear refreshed project tags: %w", err)
	}
	if err = insertProjectTags(ctx, tx, project.ID, project.Tags); err != nil {
		return err
	}
	if err = insertManifestProposal(ctx, tx, proposal); err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]string{"sourceProposalId": sourceID})
	if err != nil {
		return fmt.Errorf("encode proposal refresh audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events
		(event_type, actor_type, actor_id, project_id, idempotency_key, detail_json, occurred_at)
		VALUES ('manifest.proposal.refreshed', ?, ?, ?, ?, ?, ?)`, actor.Type, actor.ID, project.ID, proposal.ID, detail, formatTime(proposal.CreatedAt)); err != nil {
		return fmt.Errorf("audit proposal refresh: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit proposal refresh: %w", err)
	}
	return nil
}

func insertProjectTags(ctx context.Context, tx *sql.Tx, projectID string, tags []string) error {
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_tags (project_id, tag) VALUES (?, ?)`, projectID, tag); err != nil {
			return fmt.Errorf("insert project tag: %w", err)
		}
	}
	return nil
}

func insertManifestProposal(ctx context.Context, tx *sql.Tx, proposal discovery.Proposal) error {
	candidate, err := json.Marshal(proposal.Candidate)
	if err != nil {
		return fmt.Errorf("encode manifest proposal candidate: %w", err)
	}
	confidence, err := json.Marshal(proposal.ConfidenceByField)
	if err != nil {
		return fmt.Errorf("encode manifest proposal confidence: %w", err)
	}
	unresolved, err := json.Marshal(proposal.Unresolved)
	if err != nil {
		return fmt.Errorf("encode manifest proposal unresolved fields: %w", err)
	}
	validation, err := json.Marshal(proposal.Validation)
	if err != nil {
		return fmt.Errorf("encode manifest proposal validation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO manifest_proposals
		(id, project_id, scanner_version, schema_version, candidate_json, confidence_json, unresolved_json, validation_json, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, proposal.ID, proposal.ProjectID, proposal.ScannerVersion,
		proposal.SchemaVersion, candidate, confidence, unresolved, validation, proposal.Status, formatTime(proposal.CreatedAt)); err != nil {
		return fmt.Errorf("insert manifest proposal: %w", err)
	}
	for _, item := range proposal.Evidence {
		warnings, encodeErr := json.Marshal(item.Warnings)
		if encodeErr != nil {
			return fmt.Errorf("encode discovery evidence warnings: %w", encodeErr)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO discovery_evidence
			(id, proposal_id, scanner, kind, source_path, start_line, end_line, confidence, data_json, warnings_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, proposal.ID, item.Scanner, item.Kind, item.SourcePath,
			item.Location.StartLine, item.Location.EndLine, item.Confidence, item.Data, warnings); err != nil {
			return fmt.Errorf("insert discovery evidence: %w", err)
		}
	}
	return nil
}
