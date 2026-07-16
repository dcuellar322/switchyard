import type {
  GitState,
  Project,
  ProjectActions,
  RuntimeMetricSample,
  RuntimeObservation,
} from "../../api/generated/types.gen";
import {
  loadProjectActions,
  loadProjectGit,
  loadProjectMetrics,
  loadProjectRuntime,
} from "../projects/api";

export type ProjectSnapshot = {
  project: Project;
  runtime?: RuntimeObservation;
  git?: GitState;
  metrics?: Array<RuntimeMetricSample>;
  actions?: ProjectActions;
  warnings: Array<string>;
};

export async function loadProjectSnapshot(
  project: Project,
): Promise<ProjectSnapshot> {
  const [runtime, git, metrics, actions] = await Promise.allSettled([
    loadProjectRuntime(project.id),
    loadProjectGit(project.id),
    loadProjectMetrics(project.id),
    loadProjectActions(project.id),
  ]);
  const warnings: Array<string> = [];
  if (runtime.status === "rejected")
    warnings.push("Runtime observation unavailable");
  if (git.status === "rejected") warnings.push("Git observation unavailable");
  if (metrics.status === "rejected")
    warnings.push("Resource samples unavailable");
  if (actions.status === "rejected") warnings.push("Quick actions unavailable");
  return {
    project,
    runtime: runtime.status === "fulfilled" ? runtime.value : undefined,
    git: git.status === "fulfilled" ? git.value : undefined,
    metrics: metrics.status === "fulfilled" ? metrics.value : undefined,
    actions: actions.status === "fulfilled" ? actions.value : undefined,
    warnings,
  };
}

export async function loadProjectSnapshots(
  projects: Array<Project>,
  concurrency = 6,
): Promise<Array<ProjectSnapshot>> {
  const snapshots: Array<ProjectSnapshot> = [];
  for (let index = 0; index < projects.length; index += concurrency) {
    snapshots.push(
      ...(await Promise.all(
        projects.slice(index, index + concurrency).map(loadProjectSnapshot),
      )),
    );
  }
  return snapshots;
}
