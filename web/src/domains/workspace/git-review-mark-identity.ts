import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
import type {
  GitReviewArea,
  GitReviewFileState,
  GitReviewMarkIdentity,
} from "../../contracts/schemas";

export function reviewSlotKey(area: string, path: string): string {
  return `${area}\x00${path}`;
}

export function workingChangeReviewArea(
  change: GitWorkingChange,
): GitReviewArea {
  return change.group === "staged" ? "staged" : "unstaged";
}

function parseStageIdentityKey(identityKey: string): {
  oldPath?: string;
  baseMode?: string;
  baseIdentity?: string;
  targetMode?: string;
  targetIdentity?: string;
} {
  if (!identityKey) return {};
  const parts = identityKey.split("|");
  if (parts.length < 10) return {};
  const [
    ,
    oldPath,
    ,
    ,
    ,
    baseOid,
    baseMode,
    targetOid,
    targetSha256,
    targetMode,
  ] = parts;
  return {
    ...(oldPath ? { oldPath } : {}),
    ...(baseMode ? { baseMode } : {}),
    ...(baseOid ? { baseIdentity: baseOid } : {}),
    ...(targetMode ? { targetMode } : {}),
    ...(targetOid || targetSha256
      ? { targetIdentity: targetOid || targetSha256 }
      : {}),
  };
}

export function matchesServerUncommittedReview(
  change: GitWorkingChange,
  item: GitReviewFileState | undefined,
): boolean {
  if (!item || !item.reviewed) return false;
  const parsed = parseStageIdentityKey(change.identityKey);
  if (
    parsed.targetIdentity &&
    item.targetIdentity &&
    parsed.targetIdentity !== item.targetIdentity
  ) {
    return false;
  }
  if (
    parsed.baseIdentity &&
    item.baseIdentity &&
    parsed.baseIdentity !== item.baseIdentity
  ) {
    return false;
  }
  return true;
}

export function buildUncommittedReviewMarkIdentity(
  change: GitWorkingChange,
  live: GitReviewFileState | undefined,
): GitReviewMarkIdentity {
  const area = workingChangeReviewArea(change);
  const parsed = parseStageIdentityKey(change.identityKey);
  return {
    scope: "uncommitted",
    area,
    path: change.path,
    ...(change.oldPath || parsed.oldPath || live?.oldPath
      ? { oldPath: change.oldPath || parsed.oldPath || live?.oldPath }
      : {}),
    ...((parsed.baseMode ?? live?.baseMode)
      ? { baseMode: parsed.baseMode ?? live?.baseMode }
      : {}),
    ...((parsed.baseIdentity ?? live?.baseIdentity)
      ? { baseIdentity: parsed.baseIdentity ?? live?.baseIdentity }
      : {}),
    ...((parsed.targetMode ?? live?.targetMode)
      ? { targetMode: parsed.targetMode ?? live?.targetMode }
      : {}),
    ...((parsed.targetIdentity ?? live?.targetIdentity)
      ? { targetIdentity: parsed.targetIdentity ?? live?.targetIdentity }
      : {}),
  };
}

export function buildTaskReviewMarkIdentity(
  live: GitReviewFileState,
): GitReviewMarkIdentity {
  return {
    scope: "task",
    area: "task",
    path: live.path,
    ...(live.oldPath ? { oldPath: live.oldPath } : {}),
    ...(live.baseMode ? { baseMode: live.baseMode } : {}),
    ...(live.baseIdentity ? { baseIdentity: live.baseIdentity } : {}),
    ...(live.targetMode ? { targetMode: live.targetMode } : {}),
    ...(live.targetIdentity ? { targetIdentity: live.targetIdentity } : {}),
  };
}
