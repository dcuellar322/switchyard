import { useMutation, useQuery } from "@tanstack/vue-query";
import { computed, ref } from "vue";

import type { RuntimeAction } from "../../../api/generated/types.gen";
import { isActiveState } from "../../../lib/format";
import { trackOperation } from "../../operations/store";
import { loadPortRegistry } from "../../ports/api";
import {
  loadProjects,
  runProjectAction,
  runRuntimeAction,
} from "../../projects/api";
import { loadRecentProjects, markProjectAccess } from "../../projects/recent";
import { useHostObservation } from "../../system/composables/useHostObservation";
import { loadProjectSnapshots, type ProjectSnapshot } from "../api";

export function useDashboard() {
  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: loadProjects,
    refetchInterval: 15_000,
  });
  const projectList = computed(() => projects.data.value ?? []);
  const snapshots = useQuery({
    queryKey: computed(() => [
      "dashboard-snapshots",
      ...projectList.value.map((project) => project.id),
    ]),
    queryFn: () => loadProjectSnapshots(projectList.value),
    enabled: computed(() => projectList.value.length > 0),
    refetchInterval: 15_000,
  });
  const ports = useQuery({
    queryKey: ["ports"],
    queryFn: loadPortRegistry,
    refetchInterval: 10_000,
  });
  const host = useHostObservation();
  const search = ref("");
  const statusFilter = ref("all");
  const tagFilter = ref("all");
  const sortOrder = ref("recent");
  const operationError = ref("");
  const pendingProject = ref("");

  const runtimeMutation = useMutation({
    mutationFn: ({
      projectId,
      action,
    }: {
      projectId: string;
      action: RuntimeAction;
    }) => runRuntimeAction(projectId, action),
    onSuccess: trackOperation,
  });
  const terminalMutation = useMutation({
    mutationFn: (projectId: string) => runProjectAction(projectId, "terminal"),
    onSuccess: trackOperation,
  });

  const allSnapshots = computed<Array<ProjectSnapshot>>(
    () =>
      snapshots.data.value ??
      projectList.value.map((project) => ({
        project,
        warnings: ["Loading project observations"],
      })),
  );
  const tags = computed(() =>
    [...new Set(projectList.value.flatMap((project) => project.tags))].sort(),
  );
  const recent = computed(loadRecentProjects);
  const visibleSnapshots = computed(() => {
    const query = search.value.trim().toLowerCase();
    const result = allSnapshots.value.filter((snapshot) => {
      const state = snapshot.runtime?.state ?? "unknown";
      const matchesSearch =
        !query ||
        `${snapshot.project.displayName} ${snapshot.project.slug} ${snapshot.project.tags.join(" ")}`
          .toLowerCase()
          .includes(query);
      const matchesStatus =
        statusFilter.value === "all" ||
        (statusFilter.value === "running"
          ? isActiveState(state)
          : state === statusFilter.value);
      const matchesTag =
        tagFilter.value === "all" ||
        snapshot.project.tags.includes(tagFilter.value);
      return matchesSearch && matchesStatus && matchesTag;
    });
    return result.sort((left, right) => {
      if (sortOrder.value === "name")
        return left.project.displayName.localeCompare(
          right.project.displayName,
        );
      if (sortOrder.value === "status")
        return (left.runtime?.state ?? "unknown").localeCompare(
          right.runtime?.state ?? "unknown",
        );
      return (
        Date.parse(recent.value[right.project.id] ?? right.project.updatedAt) -
        Date.parse(recent.value[left.project.id] ?? left.project.updatedAt)
      );
    });
  });
  const runningCount = computed(
    () =>
      allSnapshots.value.filter((snapshot) =>
        isActiveState(snapshot.runtime?.state),
      ).length,
  );
  const serviceCount = computed(() =>
    allSnapshots.value.reduce(
      (total, snapshot) =>
        total +
        (snapshot.runtime?.services.filter(
          (service) => service.state === "running",
        ).length ?? 0),
      0,
    ),
  );
  const memoryBytes = computed(() =>
    allSnapshots.value.reduce(
      (total, snapshot) =>
        total +
        (snapshot.metrics?.reduce(
          (subtotal, metric) => subtotal + metric.memoryBytes,
          0,
        ) ?? 0),
      0,
    ),
  );
  const repoAttention = computed(
    () =>
      allSnapshots.value.filter((snapshot) => {
        const git = snapshot.git;
        return (
          git &&
          (git.behind > 0 ||
            git.changes.staged +
              git.changes.modified +
              git.changes.untracked +
              git.changes.conflicted >
              0)
        );
      }).length,
  );
  const partialCount = computed(
    () =>
      allSnapshots.value.filter((snapshot) => snapshot.warnings.length > 0)
        .length,
  );

  async function runLifecycle(
    snapshot: ProjectSnapshot,
    action: RuntimeAction,
  ) {
    pendingProject.value = snapshot.project.id;
    operationError.value = "";
    try {
      await runtimeMutation.mutateAsync({
        projectId: snapshot.project.id,
        action,
      });
    } catch (cause) {
      operationError.value =
        cause instanceof Error
          ? cause.message
          : "The lifecycle operation could not be queued.";
    } finally {
      pendingProject.value = "";
    }
  }

  async function openTerminal(snapshot: ProjectSnapshot) {
    pendingProject.value = snapshot.project.id;
    operationError.value = "";
    try {
      await terminalMutation.mutateAsync(snapshot.project.id);
    } catch (cause) {
      operationError.value =
        cause instanceof Error
          ? cause.message
          : "The terminal action could not be queued.";
    } finally {
      pendingProject.value = "";
    }
  }

  function clearFilters() {
    search.value = "";
    statusFilter.value = "all";
    tagFilter.value = "all";
  }

  return {
    projects,
    projectList,
    snapshots,
    ports,
    host,
    search,
    statusFilter,
    tagFilter,
    sortOrder,
    operationError,
    pendingProject,
    tags,
    visibleSnapshots,
    runningCount,
    serviceCount,
    memoryBytes,
    repoAttention,
    partialCount,
    runLifecycle,
    openTerminal,
    clearFilters,
    markProjectAccess,
  };
}
