import { useMutation, useQuery } from "@tanstack/vue-query";
import { computed, ref, watch, type ComputedRef } from "vue";

import type {
  ActionDefinition,
  RuntimeAction,
  RuntimeLogEntry,
} from "../../../api/generated/types.gen";
import { isActiveState } from "../../../lib/format";
import {
  loadProjectEnvironments,
  registerEnvironments,
} from "../../environments/api";
import { trackOperation } from "../../operations/store";
import { loadPortRegistry } from "../../ports/api";
import {
  loadEffectiveManifest,
  loadProject,
  loadProjectActions,
  loadProjectGit,
  loadProjectHealth,
  loadProjectLogs,
  loadProjectMetrics,
  loadProjectRuntime,
  runProjectAction,
  runRuntimeAction,
} from "../api";
import { markProjectAccess } from "../recent";
import { useProjectLogStream } from "./useProjectLogStream";

export function useProjectDetail(projectId: ComputedRef<string>) {
  const operationError = ref("");
  const liveLogs = ref<Array<RuntimeLogEntry>>([]);
  const project = useQuery({
    queryKey: computed(() => ["project", projectId.value]),
    queryFn: () => loadProject(projectId.value),
  });
  const runtime = useQuery({
    queryKey: computed(() => ["project-runtime", projectId.value]),
    queryFn: () => loadProjectRuntime(projectId.value),
    refetchInterval: 5_000,
  });
  const health = useQuery({
    queryKey: computed(() => ["project-health", projectId.value]),
    queryFn: () => loadProjectHealth(projectId.value),
    refetchInterval: 5_000,
  });
  const logs = useQuery({
    queryKey: computed(() => ["project-logs", projectId.value]),
    queryFn: () => loadProjectLogs(projectId.value),
    refetchInterval: 10_000,
  });
  const metrics = useQuery({
    queryKey: computed(() => ["project-metrics", projectId.value]),
    queryFn: () => loadProjectMetrics(projectId.value),
    refetchInterval: 5_000,
  });
  const git = useQuery({
    queryKey: computed(() => ["project-git", projectId.value]),
    queryFn: () => loadProjectGit(projectId.value),
    refetchInterval: 10_000,
  });
  const actions = useQuery({
    queryKey: computed(() => ["project-actions", projectId.value]),
    queryFn: () => loadProjectActions(projectId.value),
  });
  const manifest = useQuery({
    queryKey: computed(() => ["project-manifest", projectId.value]),
    queryFn: () => loadEffectiveManifest(projectId.value),
  });
  const ports = useQuery({
    queryKey: ["ports"],
    queryFn: loadPortRegistry,
    refetchInterval: 10_000,
  });
  const environments = useQuery({
    queryKey: computed(() => ["project-environments", projectId.value]),
    queryFn: () => loadProjectEnvironments(projectId.value),
    refetchInterval: 10_000,
  });

  watch(projectId, markProjectAccess, { immediate: true });
  watch(
    () => logs.data.value,
    (entries) => {
      if (entries) liveLogs.value = entries.slice(-300);
    },
    { immediate: true },
  );
  const logConnection = useProjectLogStream(projectId, (entry) => {
    if (liveLogs.value.some((item) => item.sequence === entry.sequence)) return;
    liveLogs.value = [...liveLogs.value, entry].slice(-300);
  });

  const lifecycle = useMutation({
    mutationFn: ({ action, profiles }: { action: RuntimeAction; profiles: string[] }) =>
      runRuntimeAction(projectId.value, action, profiles),
    onSuccess: trackOperation,
  });
  const customAction = useMutation({
    mutationFn: (action: ActionDefinition) =>
      runProjectAction(projectId.value, action.id),
    onSuccess: trackOperation,
  });
  const registerWorktrees = useMutation({
    mutationFn: () => registerEnvironments(projectId.value),
    onSuccess: async () => {
      await Promise.all([environments.refetch(), git.refetch()]);
    },
  });

  const state = computed(() => runtime.data.value?.state ?? "unknown");
  const active = computed(() => isActiveState(state.value));
  const stateTone = computed(() =>
    ["degraded", "failed", "partially_running"].includes(state.value)
      ? "warning"
      : active.value
        ? "ready"
        : "neutral",
  );
  const availableMetrics = computed(
    () => metrics.data.value?.filter((item) => item.memoryAvailable) ?? [],
  );
  const memory = computed(() =>
    availableMetrics.value.reduce((sum, item) => sum + item.memoryBytes, 0),
  );
  const cpu = computed(
    () =>
      metrics.data.value?.reduce(
        (sum, item) => sum + (item.cpuAvailable ? item.cpuPercent : 0),
        0,
      ) ?? 0,
  );
  const cpuAvailable = computed(() =>
    Boolean(metrics.data.value?.some((item) => item.cpuAvailable)),
  );
  const memoryAvailable = computed(() => availableMetrics.value.length > 0);
  const memoryLimit = computed(() =>
    Math.max(0, ...availableMetrics.value.map((item) => item.memoryLimit)),
  );
  const changes = computed(() => {
    const value = git.data.value?.changes;
    return value
      ? value.staged + value.modified + value.untracked + value.conflicted
      : 0;
  });
  const projectPorts = computed(
    () =>
      ports.data.value?.facts.filter(
        (fact) => fact.projectId === projectId.value,
      ) ?? [],
  );
  const recentLogs = computed(() => liveLogs.value.slice(-80));
  const terminalAction = computed(() =>
    actions.data.value?.actions.find(
      (action) => action.id === "terminal" || action.type === "terminal.open",
    ),
  );
  const browserAction = computed(() =>
    actions.data.value?.actions.find(
      (action) => action.type === "browser.open",
    ),
  );
  const quickActions = computed(
    () =>
      actions.data.value?.actions
        .filter(
          (action) =>
            ![terminalAction.value?.id, browserAction.value?.id].includes(
              action.id,
            ),
        )
        .slice(0, 5) ?? [],
  );
  const requiredHealth = computed(
    () => health.data.value?.results.filter((item) => item.required) ?? [],
  );
  const isPartial = computed(() =>
    [runtime, health, metrics, git, actions].some(
      (query) => query.isError.value,
    ),
  );

  async function runLifecycle(action: RuntimeAction, profiles: string[] = []) {
    operationError.value = "";
    try {
      await lifecycle.mutateAsync({ action, profiles });
    } catch (cause) {
      operationError.value =
        cause instanceof Error
          ? cause.message
          : "The lifecycle operation could not be queued.";
    }
  }

  async function runAction(action: ActionDefinition | undefined) {
    if (!action) return;
    operationError.value = "";
    try {
      await customAction.mutateAsync(action);
    } catch (cause) {
      operationError.value =
        cause instanceof Error
          ? cause.message
          : "The trusted action could not be queued.";
    }
  }

  return {
    project,
    runtime,
    health,
    metrics,
    git,
    actions,
    manifest,
    environments,
    operationError,
    liveLogs,
    logConnection,
    lifecycle,
    customAction,
    registerWorktrees,
    state,
    active,
    stateTone,
    memory,
    cpu,
    cpuAvailable,
    memoryAvailable,
    memoryLimit,
    changes,
    projectPorts,
    recentLogs,
    terminalAction,
    browserAction,
    quickActions,
    requiredHealth,
    isPartial,
    runLifecycle,
    runAction,
  };
}
