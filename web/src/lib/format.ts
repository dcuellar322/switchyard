export function formatBytes(value: number | undefined, precision = 1): string {
  if (value === undefined || !Number.isFinite(value)) return "—";
  if (value === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const order = Math.min(
    Math.floor(Math.log(Math.abs(value)) / Math.log(1024)),
    units.length - 1,
  );
  const amount = value / 1024 ** order;
  return `${amount.toFixed(order === 0 ? 0 : precision)} ${units[order]!}`;
}

export function projectInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "SY";
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase();
  return `${parts[0]![0]}${parts.at(-1)?.[0] ?? ""}`.toUpperCase();
}

export function stateLabel(value: string | undefined): string {
  if (!value) return "Unknown";
  return value
    .replaceAll("_", " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function isActiveState(value: string | undefined): boolean {
  return [
    "starting",
    "running",
    "running_external",
    "partially_running",
    "degraded",
    "paused",
    "stopping",
  ].includes(value ?? "");
}

export function isTerminalOperation(value: string): boolean {
  return ["succeeded", "failed", "cancelled", "partially_succeeded"].includes(
    value,
  );
}

export type TagTone = "blue" | "cyan" | "green" | "orange" | "purple" | "neutral";

export function tagTone(value: string): TagTone {
  const tag = value.trim().toLowerCase();
  if (/(front|web|ui|client|vite|vue|react|marketing)/.test(tag)) return "purple";
  if (/(back|api|server|daemon|worker|go|python|node)/.test(tag)) return "blue";
  if (/(postgre|mysql|maria|sqlite|database|\bdb\b)/.test(tag)) return "green";
  if (/(redis|cache|queue|broker)/.test(tag)) return "orange";
  if (/(proxy|gateway|router|network)/.test(tag)) return "cyan";
  return "neutral";
}
