import { m } from "$lib/paraglide/messages.js";
import type {
  GitCommitPreview,
  GitCommitResult,
  Tag,
} from "../../contracts/schemas";
import type {
  GitBranchItem,
  GitCommitOutcome,
  GitCommitPreviewModel,
} from "../../contracts/git-workbench-schemas";

export function mapGitTagItem(tag: Tag): GitBranchItem {
  return {
    name: tag.name,
    kind: "tag",
    head: tag.head,
    shortHead: tag.head.slice(0, 7),
    subject: tag.subject,
    committedAt: tag.committedAt,
    favorite: false,
    upstreamGone: false,
    mergedIntoDefault: false,
    tagKind: tag.kind,
    tagOid: tag.tagOid,
    tagMessage: tag.message,
    taggerName: tag.taggerName,
    taggerEmail: tag.taggerEmail,
    taggedAt: tag.taggedAt,
    tagSigned: tag.signed,
  };
}

export function mapCommitPreviewAdvancedFields(
  preview: GitCommitPreview,
): Pick<
  GitCommitPreviewModel,
  | "headMessage"
  | "headParents"
  | "headPush"
  | "amend"
  | "amendPushed"
  | "authorOverride"
  | "authorOverridden"
  | "envAuthorName"
  | "envAuthorEmail"
  | "signoff"
  | "signoffTrailer"
> {
  const overrideStr =
    preview.authorOverride ||
    (preview.identity.authorOverride
      ? `${preview.identity.authorName} <${preview.identity.authorEmail}>`
      : "");
  return {
    headMessage: preview.headMessage ?? "",
    headParents: preview.headParents,
    headPush: preview.headPush,
    amend: preview.amend,
    amendPushed: preview.amendPushed,
    authorOverride: overrideStr,
    authorOverridden: Boolean(
      preview.identity.authorOverride || preview.authorOverride,
    ),
    envAuthorName: preview.identity.envAuthorName ?? "",
    envAuthorEmail: preview.identity.envAuthorEmail ?? "",
    signoff: preview.signoff,
    signoffTrailer: preview.identity.signoffTrailer ?? "",
  };
}

function formatCommitOutcomeSummary(result: GitCommitResult): string {
  const oid = result.commitOid?.slice(0, 7) ?? "";
  if (result.outcome === "matched") {
    if (result.amended) {
      return result.subject
        ? m.s7_v2_amended_with_subject({ oid, subject: result.subject })
        : m.s7_v2_amended({ oid });
    }
    return result.subject
      ? m.s7_op_committed_with_subject({ oid, subject: result.subject })
      : m.s7_op_committed({ oid });
  }
  if (result.outcome === "needs_review") {
    return result.amended
      ? m.s7_v2_amend_needs_review()
      : m.s7_v2_commit_needs_review();
  }
  return m.s7_v2_commit_uncertain();
}

export function mapV2CommitResultResponse(
  result: GitCommitResult,
): GitCommitOutcome {
  return {
    operationId: result.operationId,
    status: result.outcome,
    commitOid: result.commitOid,
    expectedTreeOid: result.expectedTreeOid,
    actualTreeOid: result.treeOid,
    summary: formatCommitOutcomeSummary(result),
    reasons: result.reviewReasons,
  };
}

const AUTHOR_OVERRIDE_PATTERN = /^([^<>]+?)\s*<([^<>\s]+@[^<>\s]+)>$/;

export function validateAuthorOverrideInput(raw: string): {
  valid: boolean;
  error: string;
} {
  const trimmed = raw.trim();
  if (!trimmed) return { valid: true, error: "" };
  const match = AUTHOR_OVERRIDE_PATTERN.exec(trimmed);
  if (!match) {
    return {
      valid: false,
      error: m.s7_author_override_invalid_format(),
    };
  }
  const name = (match[1] ?? "").trim();
  const email = (match[2] ?? "").trim();
  if (!name || !email) {
    return {
      valid: false,
      error: m.s7_author_override_empty_fields(),
    };
  }
  if (name.startsWith("-") || email.startsWith("-")) {
    return {
      valid: false,
      error: m.s7_author_override_leading_hyphen(),
    };
  }
  return { valid: true, error: "" };
}

export function containsRedactedRemoteMask(url: string): boolean {
  return url.includes("***");
}

export interface StashPreviewDialogBaseProps {
  loading?: boolean;
  submitting?: boolean;
  acknowledgeSessions?: boolean;
  dialogError?: string;
  onAcknowledgeSessionsChange: (checked: boolean) => void;
  onConfirm: () => void;
  onClose: () => void;
}

export interface PreviewableMutationShape {
  confirmationId: string;
  blockers: readonly unknown[];
  sessions: readonly unknown[];
}

export function canExecutePreviewedMutation(options: {
  loading: boolean;
  submitting: boolean;
  confirmationId?: string;
  blockerCount: number;
  sessionCount?: number;
  acknowledgeSessions?: boolean;
}): boolean {
  if (options.loading || options.submitting) return false;
  if (!options.confirmationId || options.blockerCount > 0) return false;
  if ((options.sessionCount ?? 0) > 0 && !options.acknowledgeSessions) {
    return false;
  }
  return true;
}

export function canConfirmPreviewModel(
  preview: PreviewableMutationShape | null | undefined,
  loading: boolean,
  submitting: boolean,
  acknowledgeSessions: boolean,
): boolean {
  return canExecutePreviewedMutation({
    loading,
    submitting,
    confirmationId: preview?.confirmationId,
    blockerCount: preview?.blockers.length ?? 0,
    sessionCount: preview?.sessions.length ?? 0,
    acknowledgeSessions,
  });
}
