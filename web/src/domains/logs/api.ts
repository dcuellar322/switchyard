import type { Project, RuntimeLogEntry } from "../../api/generated/types.gen";
import { loadProjectLogs } from "../projects/api";

export type ProjectLogBatch = {
  entries: Array<RuntimeLogEntry>;
  warnings: Array<string>;
};

export function boundLogEntries(
  entries: Array<RuntimeLogEntry>,
  limit = 500,
): Array<RuntimeLogEntry> {
  return [...entries]
    .sort(
      (left, right) => Date.parse(right.timestamp) - Date.parse(left.timestamp),
    )
    .slice(0, limit);
}

export async function loadProjectLogBatches(
  projects: Array<Project>,
  concurrency = 6,
): Promise<ProjectLogBatch> {
  const entries: Array<RuntimeLogEntry> = [];
  const warnings: Array<string> = [];
  for (let index = 0; index < projects.length; index += concurrency) {
    const batch = projects.slice(index, index + concurrency);
    const results = await Promise.allSettled(
      batch.map((project) => loadProjectLogs(project.id, 100)),
    );
    results.forEach((result, resultIndex) => {
      if (result.status === "fulfilled") entries.push(...result.value);
      else
        warnings.push(
          `${batch[resultIndex]?.displayName ?? "Project"} logs unavailable`,
        );
    });
  }
  return { entries: boundLogEntries(entries), warnings };
}
