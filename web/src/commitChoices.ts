// Labels for the start-commit dropdown. A label reads like a `git log
// --oneline` row followed by when it was made, so users recognise a commit by
// its title and age instead of typing a hash.
import type { z } from "zod";
import type { CommitsSchema } from "./schemas";

export type Commit = z.infer<typeof CommitsSchema>["commits"][number];
export type CommitChoice = { value: string; label: string };

const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["week", 7 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
];
const formatter = new Intl.RelativeTimeFormat("zh-CN", { numeric: "auto" });

export function relativeTime(iso: string, now = Date.now()): string {
  const seconds = (Date.parse(iso) - now) / 1000;
  if (Number.isNaN(seconds)) return "";
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size)
      return formatter.format(Math.trunc(seconds / size), unit);
  }
  return "刚刚";
}

const shortHash = (hash: string) => hash.slice(0, 7);
const title = (subject: string) => subject || "无标题提交";

function commitLabel(commit: Commit, now = Date.now()): string {
  const age = relativeTime(commit.committedAt, now);
  return `${shortHash(commit.hash)}  ${title(commit.subject)}${age ? ` · ${age}` : ""}`;
}

/**
 * Dropdown rows for `commits` (newest first). The first row is tagged with
 * `tip`. A selected `base` outside the list (for example a freshly fetched
 * upstream tip) stays selectable as its own leading row.
 */
export function commitChoices(
  commits: Commit[],
  options: { tip: string; base: string; baseSubject: string; now?: number },
): CommitChoice[] {
  const rows = commits.map((commit, index) => ({
    value: commit.hash,
    label: `${index === 0 ? `${options.tip} · ` : ""}${commitLabel(commit, options.now)}`,
  }));
  if (!options.base || rows.some((row) => row.value === options.base))
    return rows;
  return [
    {
      value: options.base,
      label: `${shortHash(options.base)}  ${title(options.baseSubject)}`,
    },
    ...rows,
  ];
}
