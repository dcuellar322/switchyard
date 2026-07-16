const storageKey = "switchyard.recent-projects.v1";

type RecentMap = Record<string, string>;

export function loadRecentProjects(): RecentMap {
  try {
    return JSON.parse(localStorage.getItem(storageKey) ?? "{}") as RecentMap;
  } catch {
    return {};
  }
}

export function markProjectAccess(projectId: string): void {
  const recent = loadRecentProjects();
  recent[projectId] = new Date().toISOString();
  try {
    localStorage?.setItem(storageKey, JSON.stringify(recent));
  } catch {
    // Recent access is an optional browser enhancement; storage can be denied.
  }
}
