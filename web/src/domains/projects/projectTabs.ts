export const projectTabs = [
  "overview",
  "logs",
  "terminal",
  "git",
  "ports",
  "storage",
  "agents",
  "config",
] as const;

export type ProjectTab = (typeof projectTabs)[number];
