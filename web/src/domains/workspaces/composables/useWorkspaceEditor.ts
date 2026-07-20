import { computed, reactive, ref } from 'vue'

import type {
  WorkspaceDefinition,
  WorkspaceMemberRole,
  WorkspaceRecipe,
} from '../../../api/generated/types.gen'

type WorkspaceRecipeKind = WorkspaceRecipe['kind']

export interface WorkspaceMemberOption {
  id: string
  name: string
  location: string
  environment: boolean
}

export interface WorkspaceEditorProps {
  members: Array<WorkspaceMemberOption>
  saving: boolean
}

export function useWorkspaceEditor(
  props: WorkspaceEditorProps,
  save: (definition: WorkspaceDefinition) => void,
) {
  const name = ref('')
  const description = ref('')
  const policy = ref<'rollback' | 'continue'>('rollback')
  const selected = reactive(new Map<string, WorkspaceMemberRole>())
  const dependencyProject = ref('')
  const dependencyTarget = ref('')
  const dependencies = ref<Array<{ projectId: string; dependsOnProjectId: string }>>([])
  const recipes = ref<Array<WorkspaceRecipe>>([])
  const recipeName = ref('')
  const recipeKind = ref<WorkspaceRecipeKind>('open_terminal')
  const recipeProject = ref('')
  const recipeTarget = ref('')
  const selectedIds = computed(() => [...selected.keys()])
  const canSave = computed(() => name.value.trim() !== '' && selected.size > 0)
  const memberName = (id: string): string =>
    props.members.find((item) => item.id === id)?.name ?? id

  function toggle(projectId: string, checked: boolean): void {
    if (checked) selected.set(projectId, 'application')
    else {
      selected.delete(projectId)
      dependencies.value = dependencies.value.filter(
        (edge) => edge.projectId !== projectId && edge.dependsOnProjectId !== projectId,
      )
    }
  }
  function addDependency(): void {
    if (
      !dependencyProject.value ||
      !dependencyTarget.value ||
      dependencyProject.value === dependencyTarget.value
    )
      return
    if (
      !dependencies.value.some(
        (edge) =>
          edge.projectId === dependencyProject.value &&
          edge.dependsOnProjectId === dependencyTarget.value,
      )
    ) {
      dependencies.value.push({
        projectId: dependencyProject.value,
        dependsOnProjectId: dependencyTarget.value,
      })
    }
    dependencyProject.value = ''
    dependencyTarget.value = ''
  }
  function addRecipe(): void {
    if (!recipeName.value.trim()) return
    recipes.value.push({
      id: `recipe-${recipes.value.length + 1}`,
      name: recipeName.value.trim(),
      kind: recipeKind.value,
      projectId: recipeProject.value || undefined,
      target: recipeTarget.value.trim() || undefined,
      arguments: [],
      order: recipes.value.length,
    })
    recipeName.value = ''
    recipeTarget.value = ''
  }
  function submit(): void {
    if (!canSave.value) return
    const projectIds = selectedIds.value
    save({
      name: name.value.trim(),
      description: description.value.trim() || undefined,
      policy: policy.value,
      profile: 'full',
      members: projectIds.map((projectId, order) => ({
        projectId,
        role: selected.get(projectId)!,
        order,
        healthGate: true,
        healthTimeoutSeconds: 120,
      })),
      dependencies: dependencies.value,
      recipes: recipes.value,
      profiles: [
        { id: 'full', name: 'Full workspace', projectIds, maxParallel: 4, lowMemory: false },
        {
          id: 'low-memory',
          name: 'Low memory',
          description: 'Dependency-safe sequential startup',
          projectIds,
          maxParallel: 1,
          lowMemory: true,
        },
      ],
    })
  }
  return {
    name,
    description,
    policy,
    selected,
    dependencyProject,
    dependencyTarget,
    dependencies,
    recipes,
    recipeName,
    recipeKind,
    recipeProject,
    recipeTarget,
    selectedIds,
    canSave,
    memberName,
    toggle,
    addDependency,
    addRecipe,
    submit,
  }
}
