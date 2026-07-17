package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	application "switchyard.dev/switchyard/internal/catalog/application"
	discovery "switchyard.dev/switchyard/internal/discovery/domain"
)

// CreateRescanProposal stores a fresh review candidate without changing the
// currently trusted manifest snapshot. Any older unreviewed candidate is
// superseded so the project has one clear next trust decision.
func (r *CatalogRepository) CreateRescanProposal(
	ctx context.Context,
	sourceID string,
	proposal discovery.Proposal,
	actor application.MutationActor,
) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project rescan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, proposal.ProjectID).Scan(&exists); err != nil {
		return fmt.Errorf("find rescanned project: %w", err)
	}
	if exists != 1 {
		return application.ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE manifest_proposals SET status = 'superseded'
		WHERE project_id = ? AND status = 'proposed'`, proposal.ProjectID); err != nil {
		return fmt.Errorf("supersede prior rescan proposal: %w", err)
	}
	if err = insertManifestProposal(ctx, tx, proposal); err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]string{"sourceProposalId": sourceID})
	if err != nil {
		return fmt.Errorf("encode rescan audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_events
		(event_type, actor_type, actor_id, project_id, idempotency_key, detail_json, occurred_at)
		VALUES ('manifest.proposal.rescanned', ?, ?, ?, ?, ?, ?)`, actor.Type, actor.ID, proposal.ProjectID, proposal.ID, detail, formatTime(proposal.CreatedAt)); err != nil {
		return fmt.Errorf("audit project rescan: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit project rescan: %w", err)
	}
	return nil
}
