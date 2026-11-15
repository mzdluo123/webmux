import type { FileDiffMetadata, SelectedLineRange } from "@pierre/diffs";
import { z } from "zod";
import { m } from "$lib/paraglide/messages.js";
import type {
  GitChangeGroup,
  GitWorkingChange,
  GitWorktreeContext,
} from "../../contracts/git-workbench-schemas";
import {
  GitReworkItemSchema,
  type GitReworkItem,
  type GitReworkSourceKind,
  type SessionInfo,
} from "../../contracts/schemas";
import { activeHostId, store, stored } from "../../platform/storage";

const REWORK_STORAGE_KEY = "gitWorkbenchRework";
const MAX_REWORK_ITEMS_PER_TARGET = 40;
const MAX_REWORK_TARGETS = 20;
const MAX_SNIPPET_CHARS = 4000;

const ReworkBucketSchema = z.object({
  updatedAt: z.number(),
  items: z.array(GitReworkItemSchema).max(MAX_REWORK_ITEMS_PER_TARGET),
});
const ReworkMapSchema = z.record(
  z.string().min(1).max(200),
  ReworkBucketSchema,
);

const reworkCache = new Map<string, GitReworkItem[]>();

function targetKey(repositoryId: string, worktreeId?: string): string {
  return `${repositoryId}::${worktreeId?.trim() || "__repo__"}`;
}

function hostCacheKey(repositoryId: string, worktreeId?: string): string {
  return `${activeHostId() ?? "__default__"}::${targetKey(repositoryId, worktreeId)}`;
}

function newReworkItemId(): string {
  if (typeof crypto !== "undefined" && crypto.getRandomValues) {
    return Array.from(crypto.getRandomValues(new Uint8Array(8)), (b) =>
      b.toString(16).padStart(2, "0"),
    ).join("");
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function readStoredReworkMap(): Record<
  string,
  z.infer<typeof ReworkBucketSchema>
> {
  const raw = stored(REWORK_STORAGE_KEY, "");
  if (!raw) return {};
  try {
    return ReworkMapSchema.safeParse(JSON.parse(raw)).data ?? {};
  } catch {
    return {};
  }
}

export function readGitReworkItems(
  repositoryId: string,
  worktreeId?: string,
): GitReworkItem[] {
  if (!repositoryId) return [];
  const cKey = hostCacheKey(repositoryId, worktreeId);
  const cached = reworkCache.get(cKey);
  if (cached) return cached.map((item) => ({ ...item }));
  const map = readStoredReworkMap();
  const bucket = map[targetKey(repositoryId, worktreeId)];
  const items = bucket ? bucket.items.map((item) => ({ ...item })) : [];
  reworkCache.set(cKey, items);
  return items;
}

export function writeGitReworkItems(
  repositoryId: string,
  worktreeId: string | undefined,
  items: readonly GitReworkItem[],
): GitReworkItem[] {
  if (!repositoryId) return [];
  const bounded = items
    .slice(0, MAX_REWORK_ITEMS_PER_TARGET)
    .map((item) => GitReworkItemSchema.parse(item));
  reworkCache.set(hostCacheKey(repositoryId, worktreeId), bounded);

  const map = readStoredReworkMap();
  const tKey = targetKey(repositoryId, worktreeId);
  if (bounded.length === 0) {
    delete map[tKey];
  } else {
    map[tKey] = { updatedAt: Date.now(), items: bounded };
  }
  const entries = Object.entries(map).sort(
    (a, b) => b[1].updatedAt - a[1].updatedAt,
  );
  if (entries.length === 0) {
    store(REWORK_STORAGE_KEY, null);
  } else {
    store(
      REWORK_STORAGE_KEY,
      JSON.stringify(Object.fromEntries(entries.slice(0, MAX_REWORK_TARGETS))),
    );
  }
  return bounded;
}

export interface DiffSelectionResolved {
  side: "old" | "new";
  startLine: number;
  endLine: number;
  snippet: string;
}

export interface ActiveDiffReworkSelection extends DiffSelectionResolved {
  path: string;
  oldPath?: string;
  sourceKind: GitReworkSourceKind;
  versionLabel: string;
  capturedWorktreeIdentity?: string;
}

export function toReworkItemInput(
  sel: ActiveDiffReworkSelection,
  userNote: string,
): Parameters<typeof createReworkDraftItem>[0] {
  return {
    path: sel.path,
    oldPath: sel.oldPath,
    sourceKind: sel.sourceKind,
    versionLabel: sel.versionLabel,
    startLine: sel.startLine,
    endLine: sel.endLine,
    codeSnippet: sel.snippet,
    userNote,
    capturedWorktreeIdentity: sel.capturedWorktreeIdentity,
  };
}

export function extractDiffRangeSnippet(
  fileDiff: FileDiffMetadata,
  range: SelectedLineRange,
): DiffSelectionResolved | null {
  const side: "old" | "new" = range.side === "deletions" ? "old" : "new";
  const lines =
    side === "old" ? fileDiff.deletionLines : fileDiff.additionLines;
  if (!lines.length) return null;
  const rawStart = Math.min(range.start, range.end);
  const rawEnd = Math.max(range.start, range.end);
  const startLine = Math.max(1, Math.min(lines.length, rawStart));
  const endLine = Math.max(startLine, Math.min(lines.length, rawEnd));
  const rawSnippet = lines
    .slice(startLine - 1, endLine)
    .map((line) => line.replace(/\r?\n$/, ""))
    .join("\n");
  const snippet =
    rawSnippet.length > MAX_SNIPPET_CHARS
      ? `${rawSnippet.slice(0, MAX_SNIPPET_CHARS)}\n${m.s7_rework_truncated()}`
      : rawSnippet;
  return { side, startLine, endLine, snippet };
}

export function resolveStagingVersionDescriptor(options: {
  group: GitChangeGroup;
  side: "old" | "new";
  headOid: string | null;
  liveWorktreeIdentity?: string;
}): {
  sourceKind: GitReworkSourceKind;
  versionLabel: string;
  capturedWorktreeIdentity?: string;
} {
  const shortHead = options.headOid ? options.headOid.slice(0, 7) : "";
  if (options.group === "staged") {
    if (options.side === "old") {
      return {
        sourceKind: "head",
        versionLabel: shortHead
          ? m.s7_rework_ver_head_short_del({ oid: shortHead })
          : m.s7_rework_ver_head_del(),
      };
    }
    return {
      sourceKind: "index",
      versionLabel: m.s7_rework_ver_index(),
    };
  }
  if (options.side === "old") {
    return {
      sourceKind: "index",
      versionLabel: m.s7_rework_ver_index_del(),
    };
  }
  return {
    sourceKind: "worktree",
    versionLabel: m.s7_current_working_tree(),
    ...(options.liveWorktreeIdentity
      ? { capturedWorktreeIdentity: options.liveWorktreeIdentity }
      : {}),
  };
}

export function resolveSnapshotVersionDescriptor(options: {
  isTaskScope: boolean;
  side: "old" | "new";
  leftLabel: string;
  rightLabel: string;
  baseOid: string;
  targetOid: string;
  liveWorktreeIdentity?: string;
}): {
  sourceKind: GitReworkSourceKind;
  versionLabel: string;
  capturedWorktreeIdentity?: string;
} {
  if (options.isTaskScope || options.targetOid === "WORKTREE") {
    if (options.side === "old") {
      const baseText =
        options.leftLabel ||
        (options.baseOid
          ? options.baseOid.slice(0, 7)
          : m.s7_unborn_baseline());
      return {
        sourceKind: "task-base",
        versionLabel: m.s7_rework_ver_task_base_old({ base: baseText }),
      };
    }
    return {
      sourceKind: "worktree",
      versionLabel: m.s7_current_working_tree(),
      ...(options.liveWorktreeIdentity
        ? { capturedWorktreeIdentity: options.liveWorktreeIdentity }
        : {}),
    };
  }

  if (options.side === "old") {
    const baseShort =
      options.leftLabel ||
      (options.baseOid ? options.baseOid.slice(0, 7) : m.s7_empty_tree());
    return {
      sourceKind: "commit",
      versionLabel: m.s7_rework_ver_commit_old({ oid: baseShort }),
    };
  }
  const targetShort =
    options.rightLabel ||
    (options.targetOid ? options.targetOid.slice(0, 7) : m.s7_op_kind_commit());
  return {
    sourceKind: "commit",
    versionLabel: m.s7_rework_ver_commit({ oid: targetShort }),
  };
}

export function buildHunkReworkItemInput(options: {
  change: GitWorkingChange;
  hunk: GitWorkingChange["hunks"][number];
  headOid: string | null;
  liveWorktreeIdentity?: string;
}): Parameters<typeof createReworkDraftItem>[0] {
  const { change, hunk, headOid, liveWorktreeIdentity } = options;
  const useNewSide = hunk.newLines > 0;
  const startLine = Math.max(1, useNewSide ? hunk.newStart : hunk.oldStart);
  const span = Math.max(1, useNewSide ? hunk.newLines : hunk.oldLines);
  const endLine = startLine + span - 1;
  const desc = resolveStagingVersionDescriptor({
    group: change.group,
    side: useNewSide ? "new" : "old",
    headOid,
    liveWorktreeIdentity,
  });
  const snippet = hunk.lines
    .map(
      (l) =>
        `${l.kind === "add" ? "+" : l.kind === "delete" ? "-" : " "}${l.text}`,
    )
    .join("\n");
  return {
    path: change.path,
    oldPath: change.oldPath,
    sourceKind: desc.sourceKind,
    versionLabel: desc.versionLabel,
    startLine,
    endLine,
    codeSnippet: snippet,
    capturedWorktreeIdentity: desc.capturedWorktreeIdentity,
  };
}

export function createReworkDraftItem(input: {
  path: string;
  oldPath?: string;
  sourceKind: GitReworkSourceKind;
  versionLabel: string;
  startLine: number;
  endLine: number;
  codeSnippet: string;
  userNote?: string;
  capturedWorktreeIdentity?: string;
}): GitReworkItem {
  return {
    id: newReworkItemId(),
    path: input.path,
    ...(input.oldPath && input.oldPath !== input.path
      ? { oldPath: input.oldPath }
      : {}),
    sourceKind: input.sourceKind,
    versionLabel: input.versionLabel,
    startLine: input.startLine,
    endLine: input.endLine,
    codeSnippet: input.codeSnippet,
    userNote: input.userNote ?? "",
    ...(input.capturedWorktreeIdentity
      ? { capturedWorktreeIdentity: input.capturedWorktreeIdentity }
      : {}),
    createdAt: Date.now(),
  };
}

export function formatLineRangeLabel(
  startLine: number,
  endLine: number,
): string {
  return startLine === endLine ? `L${startLine}` : `L${startLine}-L${endLine}`;
}

export function formatReworkFeedbackMarkdown(
  context: GitWorktreeContext,
  items: readonly GitReworkItem[],
  stalePaths: ReadonlySet<string> = new Set(),
): string {
  const branchText =
    context.checkedOutBranch ??
    (context.headOid ? `detached@${context.headOid.slice(0, 7)}` : "unborn");
  const headerLines = [
    m.s7_rework_md_intro({
      repo: context.repositoryName,
      branch: branchText,
    }),
  ];
  if (context.worktreePath) {
    headerLines.push(
      m.s7_rework_md_worktree_path({ path: context.worktreePath }),
    );
  }

  const sections = items.map((item, idx) => {
    const range = formatLineRangeLabel(item.startLine, item.endLine);
    const renameSuffix =
      item.oldPath && item.oldPath !== item.path
        ? m.s7_rework_md_renamed_from({ oldPath: item.oldPath })
        : "";
    const nonWorktreeNotice =
      item.sourceKind !== "worktree"
        ? m.s7_rework_md_non_worktree_notice({ version: item.versionLabel })
        : stalePaths.has(item.path)
          ? m.s7_rework_md_stale_notice()
          : "";
    const fence = item.codeSnippet.includes("```") ? "````" : "```";
    const lines = [
      `### ${idx + 1}. \`${item.path}\`${renameSuffix} · ${item.versionLabel} · ${range} ${nonWorktreeNotice}`.trim(),
    ];
    if (item.codeSnippet.trim()) {
      lines.push(`${fence}\n${item.codeSnippet}\n${fence}`);
    }
    if (item.userNote.trim()) {
      lines.push(m.s7_rework_md_req_note({ note: item.userNote.trim() }));
    } else {
      lines.push(m.s7_rework_md_req_default());
    }
    return lines.join("\n");
  });

  return `${headerLines.join("\n")}\n\n${sections.join("\n\n")}`;
}

export function formatCommitDraftAssistPrompt(
  context: GitWorktreeContext,
  stagedChanges: readonly GitWorkingChange[],
): string {
  const branchText = context.checkedOutBranch ?? m.s7_current_worktree();
  const fileList = stagedChanges
    .slice(0, 30)
    .map(
      (c) =>
        `- [${c.status}] ${c.oldPath && c.oldPath !== c.path ? `${c.oldPath} -> ${c.path}` : c.path}`,
    )
    .join("\n");
  const extra =
    stagedChanges.length > 30
      ? `\n${m.s7_commit_assist_more_files({ count: String(stagedChanges.length - 30) })}`
      : "";
  return [
    m.s7_commit_assist_intro({
      repo: context.repositoryName,
      branch: branchText,
    }),
    "",
    m.s7_commit_assist_staged_header(),
    `${fileList}${extra}`,
  ].join("\n");
}

export function filterActiveWorktreeSessions(
  sessions: readonly SessionInfo[],
  worktreeId?: string,
  worktreePath?: string | null,
): SessionInfo[] {
  if (!worktreeId && !worktreePath) return [];
  return sessions.filter((s) => {
    if (s.state !== "active" || s.status === "exited") return false;
    if (worktreeId && s.workspace?.worktreeId === worktreeId) return true;
    if (worktreePath && s.cwd === worktreePath) return true;
    return false;
  });
}
