import { fireEvent, render, screen } from '@testing-library/vue'
import { expect, test, vi } from 'vitest'

import AIEvidenceConsent from '../../src/domains/projects/components/AIEvidenceConsent.vue'
import AIFieldReview from '../../src/domains/projects/components/AIFieldReview.vue'

test('shows unavailable providers and the exact redacted evidence before consent', async () => {
  const onConsent = vi.fn()
  const onStart = vi.fn()
  render(AIEvidenceConsent, {
    props: {
      providers: [
        { id: 'codex', name: 'Codex CLI', kind: 'cli', available: true, supportedBudgetKinds: ['timeout'] },
        { id: 'claude', name: 'Claude Code', kind: 'cli', available: false, reason: 'executable not found', supportedBudgetKinds: [] },
      ],
      preview: {
        bundle: {
          version: 'switchyard.dev/ai-evidence/v1alpha1', projectId: 'project-1', proposalId: 'proposal-1', deterministicCandidate: {}, confidenceByField: {}, unresolved: ['/runtime/driver'],
          evidence: [{ id: 'ev-1', kind: 'node.script', sourcePath: 'package.json', location: { startLine: 4, endLine: 4 }, confidence: 0.9, data: { command: ['pnpm', 'dev'], token: '[REDACTED]' }, warnings: [], excerpt: '"dev": "vite"', truncated: false }],
          redactionCount: 1, truncated: false, encodedBytes: 512,
        },
        encoded: { evidence: [{ id: 'ev-1', token: '[REDACTED]' }] }, sha256: 'a'.repeat(64), limits: { timeoutSeconds: 90 },
      },
      selectedProvider: 'codex', consented: false, pending: false,
      'onUpdate:consented': onConsent, onStart,
    },
  })

  expect(screen.getByText(/No repository access/)).toBeInTheDocument()
  expect(screen.getByText((_, element) => element?.tagName === 'LI' && element.textContent?.includes('Claude Code: executable not found') === true)).toBeInTheDocument()
  expect(screen.getByText('package.json:4')).toBeInTheDocument()
  expect(screen.getAllByText('1', { selector: 'strong' }).length).toBeGreaterThan(0)
  await fireEvent.click(screen.getByText('Inspect exact immutable JSON payload'))
  expect(screen.getAllByText(/REDACTED/).length).toBeGreaterThan(0)
  await fireEvent.click(screen.getByRole('checkbox'))
  expect(onConsent).toHaveBeenCalledWith(true)
  expect(screen.getByRole('button', { name: 'Generate reviewable proposal' })).toBeDisabled()
  expect(onStart).not.toHaveBeenCalled()
})

test('renders rejected fields, deterministic conflicts, and dry-run state honestly', () => {
  render(AIFieldReview, {
    props: {
      cancelling: false,
      run: {
        operationId: 'op-1', projectId: 'project-1', sourceProposalId: 'proposal-1', resultProposalId: 'proposal-2', provider: 'codex', model: 'fixture', state: 'succeeded',
        bundle: { evidence: [] }, bundleSha256: 'b'.repeat(64), limits: {},
        fields: [{ path: '/actions/steal', source: 'rejected', confidence: 0, evidenceIds: ['ev-1'], rationale: '', warnings: ['command not backed by evidence'] }],
        conflicts: [{ path: '/metadata/name', deterministicValue: 'Safe', proposedValue: 'Invented', resolution: 'kept_deterministic' }],
        warnings: ['provider action was filtered'], dryRun: { valid: false, schemaValid: true, evidenceBacked: false, repositorySafe: true, errors: [], warnings: [] }, usage: {}, startedAt: '2026-07-16T12:00:00Z', finishedAt: '2026-07-16T12:00:01Z',
      },
    },
  })

  expect(screen.getByText('Dry-run needs review')).toBeInTheDocument()
  expect(screen.getByText('/actions/steal')).toBeInTheDocument()
  expect(screen.getByText('rejected')).toBeInTheDocument()
  expect(screen.getByText('Kept deterministic value')).toBeInTheDocument()
  expect(screen.getByText(/Invented/)).toBeInTheDocument()
})
