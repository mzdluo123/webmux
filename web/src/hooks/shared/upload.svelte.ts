import { errorMessage } from "../../platform/error";
import { m } from "$lib/paraglide/messages.js";
import { onMount } from "svelte";
import { SvelteMap } from "svelte/reactivity";
import { attachFileDrop } from "./file-drop";
import {
  chatAttachmentRefusal,
  mapChatScanError,
  MAX_CHAT_ATTACHMENT_BYTES,
  MAX_CHAT_ATTACHMENTS,
  pasteDroppedSource,
  runSessionTreeUploads,
  runUploads,
  type UploadProgress,
  type UploadSend,
} from "./upload-runner";
import {
  captureDroppedSource,
  filesToDroppedSource,
  flattenUploadFiles,
  resolveUploadTree,
  type DroppedSource,
} from "../../platform/upload-entries";
import type { ChatAttachment } from "../../contracts/schemas";
import type { Connection } from "../../platform/ws-contracts";
import { allowAutomaticInputFocus } from "./input-focus";
import type {
  DraftOperation,
  InputDestination,
  TextInsertion,
} from "../../domains/terminal/terminal-draft.svelte";

export type { UploadProgress, UploadSend };

// Connection state is only required when the operation still targets the PTY.
interface UploadTerminal {
  readonly exitCode: number | null;
  readonly connection: Connection;
  readonly restoring: boolean;
  focus(): void;
}

export interface UploadInput {
  begin(): DraftOperation;
  destination(operation: DraftOperation): InputDestination;
  insert(text: string, operation: DraftOperation): TextInsertion;
}

export interface ChatUploads {
  readonly progress: UploadProgress | null;
  /** Finished uploads waiting to be sent with the next message. */
  readonly attachments: readonly ChatAttachment[];
  /** Object or data URL for an uploaded image attachment, when available locally. */
  preview?(path: string): string | undefined;
  uploadFiles(files: File[]): Promise<void>;
  uploadSource(source: DroppedSource): Promise<void>;
  filePaste(event: ClipboardEvent): boolean;
  cancel(): void;
  remove(path: string): void;
  /** Hands the pending attachments to a send and clears them. */
  take(): ChatAttachment[];
  /** Releases local previews after a send succeeds. */
  commit(attachments: ChatAttachment[]): void;
  /** Puts a failed send's attachments back in front of newer ones. */
  restore(attachments: ChatAttachment[]): void;
}

// All upload sources share the same operation identity and quoted-path insertion.
export function useUpload(
  getHost: () => HTMLElement,
  id: string,
  terminal: UploadTerminal,
  setError: (message: string) => void,
  input: UploadInput,
  available: () => boolean = () => true,
) {
  let progress = $state<UploadProgress | null>(null);
  let dragging = $state(false);
  let foreground = $state(!document.hidden);
  let controller: AbortController | undefined;
  let waiter = $state<{
    resolve: () => void;
    reject: (error: Error) => void;
    operation: DraftOperation;
  } | null>(null);
  let mounted = false;

  $effect(() => {
    const current = waiter;
    if (!current) return;
    const target = input.destination(current.operation);
    if (
      target === "failed" ||
      terminal.exitCode !== null ||
      terminal.connection === "exited" ||
      terminal.connection === "missing"
    ) {
      waiter = null;
      current.reject(new Error(m.s1_terminal_exited_or_missing()));
    } else if (
      target === "draft" ||
      (foreground && terminal.connection === "connected" && !terminal.restoring)
    ) {
      waiter = null;
      current.resolve();
    }
  });

  async function waitForDestination(
    signal: AbortSignal,
    operation: DraftOperation,
  ) {
    if (signal.aborted) {
      throw new DOMException(m.s1_operation_cancelled(), "AbortError");
    }
    const { promise, resolve, reject } = Promise.withResolvers<void>();
    const abort = () =>
      reject(new DOMException(m.s1_operation_cancelled(), "AbortError"));
    const timeout = window.setTimeout(
      () => reject(new Error(m.s1_wait_terminal_timeout())),
      30_000,
    );
    signal.addEventListener("abort", abort, { once: true });
    waiter = { resolve, reject, operation };
    try {
      await promise;
    } finally {
      clearTimeout(timeout);
      signal.removeEventListener("abort", abort);
      waiter = null;
    }
  }

  async function uploadSource(
    source: DroppedSource,
    operation = input.begin(),
  ) {
    if (!source.items.length) return;
    if (!available()) {
      setError(m.s1_upload_unavailable());
      return;
    }
    if (!mounted || input.destination(operation) === "failed") return;
    if (progress) {
      setError(m.s1_upload_in_progress_wait());
      return;
    }
    if (
      terminal.exitCode !== null ||
      terminal.connection === "exited" ||
      terminal.connection === "missing"
    ) {
      setError(m.s1_upload_terminal_exited_or_missing());
      return;
    }
    setError("");
    const current = new AbortController();
    controller = current;
    let pendingPath = "";
    const inserted = { terminal: false };
    try {
      if (source.hasDirectory) {
        progress = {
          name: source.primaryName,
          percent: 0,
          index: 0,
          total: 0,
          waiting: false,
          scanning: true,
        };
      }
      const tree = await resolveUploadTree(source, { signal: current.signal });
      if (!tree.nodes.length) return;
      await runSessionTreeUploads({
        sessionId: id,
        nodes: tree.nodes,
        fileCount: tree.fileCount,
        signal: current.signal,
        report: (value) => (progress = value),
        insert: async (path) => {
          pendingPath = path;
          try {
            await waitForDestination(current.signal, operation);
            if (!mounted || current.signal.aborted) {
              throw new DOMException(m.s1_operation_cancelled(), "AbortError");
            }
            // Quote arbitrary filenames safely. Uploading never runs a command.
            const target: TextInsertion = input.insert(
              "'" + path.replaceAll("'", "'\\''") + "' ",
              operation,
            );
            if (target === "failed") {
              throw new Error(m.s1_input_destination_not_ready());
            }
            inserted.terminal = target === "terminal";
            pendingPath = "";
          } catch (err) {
            throw new Error(
              m.s1_uploaded_but_not_inserted({
                path,
                error: errorMessage(err),
              }),
            );
          }
        },
      });
      if (
        inserted.terminal &&
        input.destination(operation) === "terminal" &&
        allowAutomaticInputFocus()
      )
        terminal.focus();
    } catch (err) {
      if (mounted || pendingPath) setError(errorMessage(err));
    } finally {
      controller = undefined;
      if (mounted) progress = null;
    }
  }

  function uploadFiles(files: File[], operation = input.begin()) {
    return uploadSource(filesToDroppedSource(files), operation);
  }

  // Returns whether the paste carried files or folders and was taken over as an upload.
  function filePaste(event: ClipboardEvent) {
    return available() ? pasteDroppedSource(event, uploadSource) : false;
  }

  onMount(() => {
    mounted = true;
    // bind:this is cleared before onMount teardown; retain the listener owner.
    const host = getHost();
    let foregroundFrame = 0;
    const visibilityChanged = () => {
      cancelAnimationFrame(foregroundFrame);
      foreground = false;
      // Let foreground reconnect handlers invalidate the old socket first.
      if (!document.hidden)
        foregroundFrame = requestAnimationFrame(() => {
          foreground = !document.hidden;
        });
    };
    document.addEventListener("visibilitychange", visibilityChanged);
    // Capture before xterm consumes paste/drop events; text still uses xterm.
    const detachDrop = attachFileDrop(host, {
      exclusive: true,
      onEnter: () => {
        dragging = true;
      },
      onOver: (transfer) => {
        transfer.dropEffect = !available() || progress ? "none" : "copy";
      },
      onLeave: () => {
        dragging = false;
      },
      onDrop: (transfer) => {
        dragging = false;
        void uploadSource(captureDroppedSource(transfer));
      },
    });
    host.addEventListener("paste", filePaste, true);
    return () => {
      document.removeEventListener("visibilitychange", visibilityChanged);
      cancelAnimationFrame(foregroundFrame);
      mounted = false;
      controller?.abort();
      host.removeEventListener("paste", filePaste, true);
      detachDrop();
    };
  });

  return {
    get progress() {
      return progress;
    },
    get dragging() {
      return dragging;
    },
    uploadFiles,
    uploadSource,
    filePaste,
    cancel() {
      controller?.abort();
    },
  };
}

// The kinds the server sends as native image blocks; the server sniffs the
// stored bytes again and its answer is what the timeline shows.
const imageTypes: Record<string, true> = {
  "image/png": true,
  "image/jpeg": true,
  "image/gif": true,
  "image/webp": true,
};

export interface ChatAttachmentStore {
  get(): ChatAttachment[];
  set(next: ChatAttachment[]): void;
}

// Chat uploads become attachments of the next message rather than text in
// the draft: the server sends them as ACP content blocks, or as server paths
// when the agent cannot take them. send stores each file: in a session's
// uploads, or in the quick start staging folder before a session exists.
export function useChatUpload(
  send: UploadSend,
  setError: (message: string) => void,
  store?: ChatAttachmentStore,
): ChatUploads {
  let progress = $state<UploadProgress | null>(null);
  let localAttachments = $state<ChatAttachment[]>([]);
  const previews = new SvelteMap<string, string>();
  let controller: AbortController | undefined;
  let disposed = false;

  const currentAttachments = () => store?.get() ?? localAttachments;
  const writeAttachments = (next: ChatAttachment[]) => {
    if (store) store.set(next);
    else localAttachments = next;
  };

  async function runChatBatch(files: File[], current: AbortController) {
    const refused = chatAttachmentRefusal(currentAttachments(), files);
    setError(refused);
    if (refused) return;
    await runUploads({
      send,
      files,
      signal: current.signal,
      waiting: false,
      report: (value) => (progress = value),
      insert: async (path, file) => {
        const isImage = Object.hasOwn(imageTypes, file.type);
        if (isImage) {
          const existing = previews.get(path);
          if (existing) URL.revokeObjectURL(existing);
          previews.set(path, URL.createObjectURL(file));
        }
        writeAttachments([
          ...currentAttachments(),
          {
            path,
            name: file.name,
            ...(file.type ? { mimeType: file.type } : {}),
            size: file.size,
            kind: isImage ? "image" : "file",
          },
        ]);
      },
    });
  }

  async function uploadSource(source: DroppedSource) {
    if (!source.items.length) return;
    if (progress) {
      setError(m.s1_upload_in_progress_wait());
      return;
    }
    setError("");
    const current = new AbortController();
    controller = current;
    try {
      const plainFiles = source.items.every((item) => item.kind === "file")
        ? source.items.map((item) => item.file)
        : null;
      if (plainFiles) {
        await runChatBatch(plainFiles, current);
        return;
      }
      if (source.hasDirectory) {
        progress = {
          name: source.primaryName,
          percent: 0,
          index: 0,
          total: 0,
          waiting: false,
          scanning: true,
        };
      }
      const remaining = Math.max(
        0,
        MAX_CHAT_ATTACHMENTS - currentAttachments().length,
      );
      const tree = await resolveUploadTree(source, {
        maxEntries: 1000,
        maxFiles: remaining,
        maxFileBytes: MAX_CHAT_ATTACHMENT_BYTES,
        skipGit: true,
        signal: current.signal,
      });
      const files = flattenUploadFiles(tree.nodes);
      if (!files.length) {
        if (tree.hadDirectory) setError(m.s1_upload_empty_folder());
        return;
      }
      await runChatBatch(files, current);
    } catch (err) {
      if (!disposed) setError(mapChatScanError(err));
    } finally {
      controller = undefined;
      progress = null;
    }
  }

  async function uploadFiles(files: File[]) {
    if (!files.length) return;
    if (progress) {
      setError(m.s1_upload_in_progress_wait());
      return;
    }
    const current = new AbortController();
    controller = current;
    try {
      await runChatBatch(files, current);
    } catch (err) {
      if (!disposed) setError(errorMessage(err));
    } finally {
      controller = undefined;
      progress = null;
    }
  }

  onMount(() => () => {
    disposed = true;
    controller?.abort();
    for (const url of previews.values()) URL.revokeObjectURL(url);
    previews.clear();
  });

  return {
    get progress() {
      return progress;
    },
    get attachments() {
      return currentAttachments();
    },
    preview(path) {
      return previews.get(path);
    },
    uploadFiles,
    uploadSource,
    filePaste(event) {
      return pasteDroppedSource(event, uploadSource);
    },
    cancel() {
      controller?.abort();
    },
    remove(path) {
      const url = previews.get(path);
      if (url) {
        URL.revokeObjectURL(url);
        previews.delete(path);
      }
      writeAttachments(
        currentAttachments().filter((item) => item.path !== path),
      );
    },
    take() {
      const taken = [...currentAttachments()];
      writeAttachments([]);
      return taken;
    },
    commit(sent) {
      for (const item of sent) {
        const url = previews.get(item.path);
        if (url) {
          URL.revokeObjectURL(url);
          previews.delete(item.path);
        }
      }
    },
    restore(sent) {
      writeAttachments([...sent, ...currentAttachments()]);
    },
  };
}
