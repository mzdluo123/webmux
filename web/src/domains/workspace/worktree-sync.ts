// How a worktree or branch stands against its upstream, and the words the
// UI uses for it. Every count is local: ahead and behind compare with the
// remote-tracking refs from the last fetch.
import { m } from "$lib/paraglide/messages.js";
import {
  remoteBranchBases,
  remoteLocalName,
  suggestWorktreeBranch,
} from "./branch-name";
import { relativeTime } from "./commit-choices";
import type { WorktreePreset } from "../sessions/worktree-preset";
import type {
  Branch,
  Branches,
  Repository,
  SwitchPreview,
  SwitchTarget,
  Worktree,
  WorktreeChanges,
  WorktreeOperation,
} from "../../contracts/schemas";

type Tracking = Pick<Branch, "upstream" | "ahead" | "behind" | "upstreamGone">;

export type UpstreamState =
  | { kind: "none" }
  | { kind: "gone"; upstream: string }
  | { kind: "uncounted"; upstream: string }
  | { kind: "tracked"; upstream: string; ahead: number; behind: number };

/** The upstream state of a branch or worktree. */
export function upstreamState(tracking: Tracking): UpstreamState {
  const { upstream, ahead, behind } = tracking;
  if (upstream === null) return { kind: "none" };
  if (tracking.upstreamGone) return { kind: "gone", upstream };
  if (ahead === null || behind === null) return { kind: "uncounted", upstream };
  return { kind: "tracked", upstream, ahead, behind };
}

/**
 * A worktree's upstream state; `unknown` when its status was not read, and
 * null where an upstream means nothing: bare, detached or without commits.
 */
export function worktreeUpstream(
  tree: Worktree,
): UpstreamState | { kind: "unknown" } | null {
  if (tree.kind === "bare") return null;
  if (!tree.available || tree.changes === null) return { kind: "unknown" };
  if (tree.detached || tree.head === null) return null;
  return upstreamState(tree);
}

/**
 * The home title's indicator shows only what needs attention: commits
 * ahead or behind, a deleted upstream, an operation in progress.
 */
export function syncNotable(tree: Worktree): boolean {
  const state = worktreeUpstream(tree);
  return (
    tree.operation !== null ||
    state?.kind === "gone" ||
    (state?.kind === "tracked" && (state.ahead > 0 || state.behind > 0))
  );
}

/**
 * Whether a worktree's branch can be switched in place: bare, missing or
 * prunable worktrees have no checkout to switch.
 */
export function switchableWorktree(tree: Worktree): boolean {
  return tree.kind !== "bare" && tree.available && !tree.prunable;
}

/**
 * Whether a worktree currently reports uncommitted tracked, untracked, or
 * conflicted changes (used to offer the direct `提交改动…` shortcut).
 */
export function canCommitWorktree(tree: Worktree | undefined): boolean {
  if (!tree) return true;
  return (
    tree.available &&
    !tree.prunable &&
    !tree.locked &&
    tree.kind !== "bare" &&
    hasWorktreeChanges(tree)
  );
}

export function hasWorktreeChanges(tree: Worktree | undefined): boolean {
  if (!tree || !tree.available || tree.prunable || tree.kind === "bare") {
    return false;
  }
  if (tree.dirty === true || tree.hasUntracked) return true;
  if (!tree.changes) return false;
  return (
    tree.changes.staged +
      tree.changes.unstaged +
      tree.changes.untracked +
      tree.changes.conflicted >
    0
  );
}

/** The words of a state without counts to show. */
export function upstreamText(
  state: UpstreamState | { kind: "unknown" },
): string {
  switch (state.kind) {
    case "none":
      return m.s8_upstream_none();
    case "gone":
      return m.s8_upstream_gone();
    case "uncounted":
      return m.s8_upstream_uncounted();
    case "unknown":
      return m.s8_unknown();
    case "tracked":
      return state.ahead || state.behind ? "" : m.s8_upstream_synced();
  }
}

export const aheadLabel = (count: number) =>
  m.s8_ahead_commits({ count: String(count) });
export const behindLabel = (count: number) =>
  m.s8_behind_commits({ count: String(count) });

export const operationLabels: Record<WorktreeOperation, string> = {
  get merge() {
    return m.s8_op_merge();
  },
  get rebase() {
    return m.s8_op_rebase();
  },
  get "cherry-pick"() {
    return m.s8_op_cherry_pick();
  },
  get revert() {
    return m.s8_op_revert();
  },
  get bisect() {
    return m.s8_op_bisect();
  },
};

/** Change counts in display order. */
export function changeCounts(changes: WorktreeChanges) {
  return [
    { label: m.s8_changes_staged(), count: changes.staged },
    { label: m.s8_changes_unstaged(), count: changes.unstaged },
    { label: m.s8_changes_untracked(), count: changes.untracked },
    { label: m.s8_changes_conflicted(), count: changes.conflicted },
  ];
}

/** The last fetch's age (labels mark it approximate), or 未知 if never. */
export function fetchedText(lastFetchedAt: string | null, now = Date.now()) {
  return lastFetchedAt ? relativeTime(lastFetchedAt, now) : m.s8_unknown();
}

export function deriveSwitchBranchItems(
  data: Branches | null,
  worktreePath: string,
  worktreeBranch: string | null,
) {
  const localNames = new Set((data?.branches ?? []).map((item) => item.name));
  const remotes = data ? remoteBranchBases(data.bases, localNames) : [];
  const localItems = (data?.branches ?? []).map((item) => {
    const elsewhere = item.worktreePaths.find((p) => p !== worktreePath);
    return {
      name: item.name,
      reason:
        item.name === worktreeBranch
          ? m.s8_switch_current_branch()
          : elsewhere
            ? m.s8_switch_checked_out_at({ path: elsewhere })
            : undefined,
    };
  });
  const remoteItems = remotes.map((item) => {
    const local = remoteLocalName(item.name);
    return {
      name: item.name,
      reason: localNames.has(local)
        ? m.s8_switch_local_exists({ branch: local })
        : undefined,
    };
  });
  return { localNames, remotes, localItems, remoteItems };
}

export function suggestSwitchBranchName(
  localNames: ReadonlySet<string>,
  repository: Repository,
  avoid?: string,
) {
  return suggestWorktreeBranch({
    branchNames: [...localNames],
    worktreePaths: repository.worktrees.map((tree) => tree.path),
    avoid,
  });
}

export function buildSwitchTarget(input: {
  mode: "existing" | "new" | "detach";
  chosen: { kind: "local" | "remote"; name: string } | null;
  newName: string;
  nameTaken: boolean;
  startPoint: string;
  commit: string;
}): SwitchTarget | null {
  if (input.mode === "existing") {
    if (!input.chosen) return null;
    return input.chosen.kind === "local"
      ? { kind: "local", branch: input.chosen.name }
      : { kind: "remote", ref: input.chosen.name };
  }
  if (input.mode === "new") {
    const name = input.newName.trim();
    return name && !input.nameTaken && input.startPoint
      ? { kind: "new", name, startPoint: input.startPoint }
      : null;
  }
  return input.commit ? { kind: "detach", commit: input.commit } : null;
}

export function canConfirmSwitch(
  preview: SwitchPreview | null,
  previewing: boolean,
  busy: boolean,
  acknowledged: boolean,
): boolean {
  return (
    !!preview &&
    !previewing &&
    !busy &&
    preview.confirmationId !== "" &&
    !preview.blockers.length &&
    (!preview.sessions.length || acknowledged)
  );
}

export function resolveSwitchOccupantPath(
  preview: SwitchPreview | null,
): string | undefined {
  return preview?.blockers.find((item) => item.code === "occupied" && item.path)
    ?.path;
}

export function resolveSwitchNewWorktreePreset(
  preview: SwitchPreview | null,
  target: SwitchTarget | null,
  startIsBranch: boolean,
): WorktreePreset | null {
  if (!preview || !target || target.kind === "detach") return null;
  const blocked = (code: string) =>
    preview.blockers.some((item) => item.code === code);
  if (!blocked("changes") && !preview.sessions.length) return null;
  if (blocked("occupied") || blocked("exists") || blocked("same")) return null;
  if (target.kind === "local")
    return { mode: "existing", branch: target.branch };
  if (target.kind === "remote")
    return {
      mode: "new",
      startBranch: target.ref,
      name: remoteLocalName(target.ref),
    };
  return {
    mode: "new",
    name: target.name,
    ...(startIsBranch ? { startBranch: target.startPoint } : {}),
  };
}
