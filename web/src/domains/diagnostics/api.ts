import {
  acknowledgeDiagnosticNotification,
  createAutomationEvaluation,
  createAutomationRecipe,
  createDiagnosticActionOperation,
  createDiagnosticFeedback,
  createProjectDiagnosis,
  getLatestProjectDiagnosis,
  listAiProposalProviders,
  listAutomationRecipes,
  listDiagnosticNotifications,
  updateAutomationRecipe,
} from '../../api/generated/sdk.gen'
import type {
  AutomationRecipe,
  CreateAutomationRecipeRequest,
  Diagnosis,
  DiagnosticNotification,
  Operation,
} from '../../api/generated/types.gen'
import { mutationHeaders } from '../session/bootstrap'

function headers(): Record<string, string> {
  return mutationHeaders(`ui_${crypto.randomUUID()}`)
}

export async function loadProviders() {
  const result = await listAiProposalProviders()
  if (result.error || !result.data) throw new Error('Optional diagnosis providers are unavailable.')
  return result.data
}

export async function loadLatestDiagnosis(projectId: string): Promise<Diagnosis | null> {
  const result = await getLatestProjectDiagnosis({ path: { projectId } })
  if (result.data) return result.data
  if (result.response?.status === 404) return null
  throw new Error('The latest diagnostic receipt is unavailable.')
}

export async function diagnoseProject(projectId: string, provider?: string): Promise<Diagnosis> {
  const result = await createProjectDiagnosis({
    path: { projectId },
    body: provider ? { provider } : {},
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('The diagnostic bundle could not be completed.')
  return result.data
}

export async function reviewHypothesis(
  diagnosisId: string,
  hypothesisId: string,
  verdict: 'accurate' | 'false_positive',
) {
  const result = await createDiagnosticFeedback({
    path: { diagnosisId },
    body: { hypothesisId, verdict },
    headers: headers(),
  })
  if (result.error || !result.data)
    throw new Error('Local diagnostic feedback could not be recorded.')
  return result.data
}

export async function runSuggestedAction(
  diagnosisId: string,
  actionId: string,
): Promise<Operation> {
  const result = await createDiagnosticActionOperation({
    path: { diagnosisId, actionId },
    headers: headers() as { 'Idempotency-Key': string },
  })
  if (result.error || !result.data)
    throw new Error('The approved diagnostic action could not be queued.')
  return result.data
}

export async function loadRecipes(projectId: string): Promise<Array<AutomationRecipe>> {
  const result = await listAutomationRecipes({ query: { projectId } })
  if (result.error || !result.data) throw new Error('Automation recipes are unavailable.')
  return result.data
}

export async function saveRecipe(
  request: CreateAutomationRecipeRequest,
): Promise<AutomationRecipe> {
  const result = await createAutomationRecipe({ body: request, headers: headers() })
  if (result.error || !result.data)
    throw new Error('The disabled automation recipe could not be saved.')
  return result.data
}

export async function setRecipeEnabled(
  recipeId: string,
  enabled: boolean,
): Promise<AutomationRecipe> {
  const result = await updateAutomationRecipe({
    path: { recipeId },
    body: { enabled },
    headers: headers(),
  })
  if (result.error || !result.data) throw new Error('The automation state could not be updated.')
  return result.data
}

export async function evaluateRecipes(projectId: string): Promise<Array<string>> {
  const result = await createAutomationEvaluation({ path: { projectId }, headers: headers() })
  if (result.error || !result.data) throw new Error('Automation evaluation failed.')
  return result.data.operationIds
}

export async function loadNotifications(projectId: string): Promise<Array<DiagnosticNotification>> {
  const result = await listDiagnosticNotifications({
    query: { projectId, includeAcknowledged: false, limit: 100 },
  })
  if (result.error || !result.data) throw new Error('Diagnostic notifications are unavailable.')
  return result.data
}

export async function acknowledgeNotification(
  notificationId: string,
): Promise<DiagnosticNotification> {
  const result = await acknowledgeDiagnosticNotification({
    path: { notificationId },
    headers: headers(),
  })
  if (result.error || !result.data)
    throw new Error('The diagnostic notification could not be acknowledged.')
  return result.data
}
