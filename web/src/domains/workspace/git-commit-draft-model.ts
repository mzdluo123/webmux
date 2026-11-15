import type { GitCommitDraft } from "../../contracts/schemas";

export interface GitCommitDraftConflictState {
  localSubject: string;
  localBody: string;
  remoteDraft: GitCommitDraft;
  message: string;
}

export function splitCommitMessageDraft(message: string): {
  subject: string;
  body: string;
} {
  const normalized = message.replace(/\r\n/g, "\n");
  const idx = normalized.indexOf("\n");
  if (idx === -1) return { subject: normalized, body: "" };
  const subject = normalized.slice(0, idx);
  const body = normalized.slice(idx + 1).replace(/^\n/, "");
  return { subject, body };
}

export function joinCommitMessageDraft(subject: string, body: string): string {
  const cleanSubject = subject.replace(/\r?\n/g, " ");
  const cleanBody = body.replace(/\r\n/g, "\n");
  if (!cleanBody.trim() && cleanBody.length === 0) return cleanSubject;
  return `${cleanSubject}\n\n${cleanBody}`;
}

export function mergeCommitMessageDrafts(
  local: { subject: string; body: string },
  remote: { subject: string; body: string },
): { subject: string; body: string } {
  const ls = local.subject.trim();
  const rs = remote.subject.trim();
  const subject = !ls
    ? remote.subject
    : !rs || ls === rs
      ? local.subject
      : `${local.subject} / ${remote.subject}`;

  const lb = local.body.trim();
  const rb = remote.body.trim();
  let body = "";
  if (!lb) body = remote.body;
  else if (!rb || lb === rb) body = local.body;
  else {
    body = `--- 本机当前输入 ---\n${local.body}\n\n--- 另一设备已保存版本 ---\n${remote.body}`;
  }
  return { subject, body };
}
