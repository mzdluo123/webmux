import { m } from "$lib/paraglide/messages.js";
import type { ChatAttachment } from "../../contracts/schemas";
import { uploadFile } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { createUploadDir } from "../../platform/upload-dir";
import {
  captureDroppedSource,
  UploadLimitError,
  type DroppedSource,
  type UploadNode,
} from "../../platform/upload-entries";

// Mirrors the server's per-message bounds so an upload that could never be
// sent is refused before any bytes move; the server still enforces them.
export const MAX_CHAT_ATTACHMENTS = 8;
export const MAX_CHAT_ATTACHMENT_BYTES = 10 << 20;
const MAX_CHAT_ATTACHMENT_TOTAL_BYTES = 20 << 20;

export function chatAttachmentRefusal(
  attachments: readonly ChatAttachment[],
  files: readonly File[],
): string {
  if (attachments.length + files.length > MAX_CHAT_ATTACHMENTS) {
    return m.s1_max_attachments_per_message({ max: MAX_CHAT_ATTACHMENTS });
  }
  const oversized = files.find((file) => file.size > MAX_CHAT_ATTACHMENT_BYTES);
  if (oversized) {
    return m.s1_attachment_exceeds_limit({
      name: oversized.name,
      mib: MAX_CHAT_ATTACHMENT_BYTES >> 20,
    });
  }
  const total =
    attachments.reduce((sum, item) => sum + (item.size ?? 0), 0) +
    files.reduce((sum, file) => sum + file.size, 0);
  if (total > MAX_CHAT_ATTACHMENT_TOTAL_BYTES) {
    return m.s1_attachments_total_exceeds_limit({
      mib: MAX_CHAT_ATTACHMENT_TOTAL_BYTES >> 20,
    });
  }
  return "";
}

export function mapChatScanError(err: unknown): string {
  if (err instanceof UploadLimitError) {
    if (err.kind === "maxFiles") {
      return m.s1_max_attachments_per_message({ max: MAX_CHAT_ATTACHMENTS });
    }
    if (err.kind === "maxFileBytes") {
      return m.s1_attachment_exceeds_limit({
        name: err.detail?.name ?? "",
        mib: MAX_CHAT_ATTACHMENT_BYTES >> 20,
      });
    }
  }
  return errorMessage(err);
}

export interface UploadProgress {
  name: string;
  percent: number;
  index: number;
  total: number;
  waiting: boolean;
  scanning?: boolean;
}

/** Stores one file on the server and returns where it landed. */
export type UploadSend = (
  file: File,
  progress: (percent: number) => void,
  signal: AbortSignal,
) => Promise<{ path: string }>;

interface UploadRun {
  send: UploadSend;
  files: File[];
  signal: AbortSignal;
  /** Progress shows "等待" between upload and insertion while the PTY settles. */
  waiting: boolean;
  report(progress: UploadProgress): void;
  insert(path: string, file: File): Promise<void>;
}

function throwCancelled(): never {
  throw new DOMException(m.s1_upload_cancelled(), "AbortError");
}

async function finishUploadedPath(
  path: string,
  progress: UploadProgress,
  waiting: boolean,
  signal: AbortSignal,
  report: (progress: UploadProgress) => void,
  insert: () => Promise<void>,
) {
  if (signal.aborted) {
    throw new Error(
      m.s1_uploaded_but_not_inserted({
        path,
        error: m.s1_operation_cancelled(),
      }),
    );
  }
  report({ ...progress, percent: 100, waiting });
  await insert();
}

// Uploads flat files sequentially with progress and cancellation.
export async function runUploads(run: UploadRun) {
  for (const [index, file] of run.files.entries()) {
    const current: UploadProgress = {
      name: file.name,
      percent: 0,
      index: index + 1,
      total: run.files.length,
      waiting: false,
    };
    run.report(current);
    const { path } = await run.send(
      file,
      (percent) => run.report({ ...current, percent }),
      run.signal,
    );
    await finishUploadedPath(
      path,
      current,
      run.waiting,
      run.signal,
      (value) => run.report(value),
      () => run.insert(path, file),
    );
  }
}

interface SessionTreeUploadRun {
  sessionId: string;
  nodes: readonly UploadNode[];
  fileCount: number;
  signal: AbortSignal;
  report(progress: UploadProgress): void;
  insert(path: string): Promise<void>;
}

// Uploads loose files and recursive folders into a session's uploads/ folder.
// Each top-level folder inserts its uploaded root directory path once all of
// its contents finish.
export async function runSessionTreeUploads(run: SessionTreeUploadRun) {
  let fileIndex = 0;

  async function uploadDir(
    node: Extract<UploadNode, { kind: "directory" }>,
    parentRel: string | undefined,
  ): Promise<string> {
    if (run.signal.aborted) throwCancelled();
    const created = await createUploadDir(
      run.sessionId,
      node.name,
      parentRel,
      run.signal,
    );
    for (const child of node.children) {
      if (run.signal.aborted) throwCancelled();
      if (child.kind === "directory") {
        await uploadDir(child, created.relativePath);
        continue;
      }
      fileIndex += 1;
      const current: UploadProgress = {
        name: child.relativePath,
        percent: 0,
        index: fileIndex,
        total: run.fileCount,
        waiting: false,
      };
      run.report(current);
      await uploadFile(
        run.sessionId,
        child.file,
        (percent) => run.report({ ...current, percent }),
        run.signal,
        created.relativePath,
      );
      if (run.signal.aborted) throwCancelled();
    }
    return created.path;
  }

  for (const node of run.nodes) {
    if (run.signal.aborted) throwCancelled();
    if (node.kind === "file") {
      fileIndex += 1;
      const current: UploadProgress = {
        name: node.relativePath,
        percent: 0,
        index: fileIndex,
        total: run.fileCount,
        waiting: false,
      };
      run.report(current);
      const { path } = await uploadFile(
        run.sessionId,
        node.file,
        (percent) => run.report({ ...current, percent }),
        run.signal,
      );
      await finishUploadedPath(
        path,
        current,
        true,
        run.signal,
        (value) => run.report(value),
        () => run.insert(path),
      );
      continue;
    }

    const rootPath = await uploadDir(node, undefined);
    await finishUploadedPath(
      rootPath,
      {
        name: node.name,
        percent: 100,
        index: Math.max(fileIndex, 1),
        total: Math.max(run.fileCount, 1),
        waiting: true,
      },
      true,
      run.signal,
      (value) => run.report(value),
      () => run.insert(rootPath),
    );
  }
}

// File and folder paste is taken over from the editor; plain text keeps its native path.
export function pasteDroppedSource(
  event: ClipboardEvent,
  upload: (source: DroppedSource) => Promise<void>,
) {
  const source = captureDroppedSource(event.clipboardData);
  if (!source.items.length) return false;
  event.preventDefault();
  event.stopImmediatePropagation();
  void upload(source);
  return true;
}
