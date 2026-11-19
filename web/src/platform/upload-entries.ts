import { m } from "$lib/paraglide/messages.js";

/** One file or directory resolved from a drag-and-drop or paste payload. */
export type UploadNode =
  | { kind: "file"; name: string; relativePath: string; file: File }
  | {
      kind: "directory";
      name: string;
      relativePath: string;
      children: UploadNode[];
    };

type CapturedDropItem =
  | { kind: "entry"; entry: FileSystemEntry }
  | { kind: "file"; file: File };

/** Synchronous snapshot of a drop or paste transfer, safe to scan asynchronously. */
export interface DroppedSource {
  readonly items: readonly CapturedDropItem[];
  readonly hasDirectory: boolean;
  readonly primaryName: string;
}

export interface ResolveUploadOptions {
  /** Maximum total entries (files + directories) allowed. */
  maxEntries?: number;
  /** Maximum number of files allowed; stops scanning early when exceeded. */
  maxFiles?: number;
  /** Maximum size in bytes allowed for a single file. */
  maxFileBytes?: number;
  /** Skip ".git" entries (case-insensitive). */
  skipGit?: boolean;
  signal?: AbortSignal;
}

export interface ResolvedUploadTree {
  readonly nodes: UploadNode[];
  readonly fileCount: number;
  readonly directoryCount: number;
  readonly skippedGit: boolean;
  readonly hadDirectory: boolean;
}

export type UploadLimitKind = "maxEntries" | "maxFiles" | "maxFileBytes";

export class UploadLimitError extends Error {
  constructor(
    public readonly kind: UploadLimitKind,
    message: string,
    public readonly detail?: { name?: string; relativePath?: string },
  ) {
    super(message);
  }
}

const DEFAULT_MAX_UPLOAD_ENTRIES = 10_000;
const DEFAULT_MAX_UPLOAD_FILE_BYTES = 100 << 20;

function checkAborted(signal?: AbortSignal) {
  if (signal?.aborted) {
    throw new DOMException(m.s1_upload_cancelled(), "AbortError");
  }
}

function isFileEntry(entry: FileSystemEntry): entry is FileSystemFileEntry {
  return (
    entry.isFile && typeof (entry as FileSystemFileEntry).file === "function"
  );
}

function isDirectoryEntry(
  entry: FileSystemEntry,
): entry is FileSystemDirectoryEntry {
  return (
    entry.isDirectory &&
    typeof (entry as FileSystemDirectoryEntry).createReader === "function"
  );
}

/** Captures file and directory entries synchronously during a drop or paste event. */
export function captureDroppedSource(
  transfer: DataTransfer | null | undefined,
): DroppedSource {
  const items: CapturedDropItem[] = [];
  let hasDirectory = false;
  let primaryName = "";

  const rawItems = transfer?.items ? Array.from(transfer.items) : [];
  for (const item of rawItems) {
    if (item.kind !== undefined && item.kind !== "file") continue;
    const entry =
      typeof item.webkitGetAsEntry === "function"
        ? item.webkitGetAsEntry()
        : null;
    if (entry) {
      if (entry.isDirectory) {
        items.push({ kind: "entry", entry });
        hasDirectory = true;
        if (!primaryName) primaryName = entry.name;
        continue;
      }
      const directFile =
        typeof item.getAsFile === "function" ? item.getAsFile() : null;
      if (directFile) {
        items.push({ kind: "file", file: directFile });
        if (!primaryName) primaryName = directFile.name;
        continue;
      }
      items.push({ kind: "entry", entry });
      if (!primaryName) primaryName = entry.name;
      continue;
    }
    const file = typeof item.getAsFile === "function" ? item.getAsFile() : null;
    if (file) {
      items.push({ kind: "file", file });
      if (!primaryName) primaryName = file.name;
    }
  }

  if (items.length === 0 && transfer?.files?.length) {
    for (const file of Array.from(transfer.files)) {
      items.push({ kind: "file", file });
      if (!primaryName) primaryName = file.name;
    }
  }

  return { items, hasDirectory, primaryName };
}

/** Wraps plain File objects (for file pickers or programmatic callers) as a DroppedSource. */
export function filesToDroppedSource(files: readonly File[]): DroppedSource {
  return {
    items: files.map((file) => ({ kind: "file", file })),
    hasDirectory: false,
    primaryName: files[0]?.name ?? "",
  };
}

/** Probes the first byte of file so unreadable local handles fail before XHR/P2P send. */
export async function ensureReadableFile(file: File): Promise<void> {
  if (typeof file.slice !== "function") return;
  try {
    await file.slice(0, 1).arrayBuffer();
  } catch {
    throw new Error(m.s1_upload_file_unreadable({ name: file.name }));
  }
}

function readEntryFile(
  entry: FileSystemFileEntry,
  relativePath: string,
): Promise<File> {
  return new Promise((resolve, reject) => {
    try {
      entry.file(resolve, () =>
        reject(new Error(m.s1_upload_file_unreadable({ name: relativePath }))),
      );
    } catch {
      reject(new Error(m.s1_upload_file_unreadable({ name: relativePath })));
    }
  });
}

async function readAllDirectoryEntries(
  dir: FileSystemDirectoryEntry,
  relativePath: string,
  signal?: AbortSignal,
): Promise<FileSystemEntry[]> {
  const reader = dir.createReader();
  const entries: FileSystemEntry[] = [];
  for (;;) {
    checkAborted(signal);
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => {
      try {
        reader.readEntries(resolve, () =>
          reject(
            new Error(m.s1_upload_folder_unreadable({ name: relativePath })),
          ),
        );
      } catch {
        reject(
          new Error(m.s1_upload_folder_unreadable({ name: relativePath })),
        );
      }
    });
    if (batch.length === 0) break;
    entries.push(...batch);
  }
  entries.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  return entries;
}

/** Recursively scans a DroppedSource into a bounded tree of directories and files. */
export async function resolveUploadTree(
  source: DroppedSource,
  options: ResolveUploadOptions = {},
): Promise<ResolvedUploadTree> {
  const maxEntries = options.maxEntries ?? DEFAULT_MAX_UPLOAD_ENTRIES;
  const maxFiles = options.maxFiles;
  const maxFileBytes = options.maxFileBytes ?? DEFAULT_MAX_UPLOAD_FILE_BYTES;
  const skipGit = options.skipGit ?? false;
  const signal = options.signal;

  let entryCount = 0;
  let fileCount = 0;
  let directoryCount = 0;
  let skippedGit = false;
  let hadDirectory = source.hasDirectory;

  const accountEntry = () => {
    entryCount += 1;
    if (entryCount > maxEntries) {
      throw new UploadLimitError(
        "maxEntries",
        m.s1_upload_too_many_entries({ max: maxEntries }),
      );
    }
  };

  const accountFile = (file: File, relativePath: string) => {
    accountEntry();
    fileCount += 1;
    if (maxFiles !== undefined && fileCount > maxFiles) {
      throw new UploadLimitError(
        "maxFiles",
        m.s1_max_attachments_per_message({ max: maxFiles }),
      );
    }
    if (file.size > maxFileBytes) {
      throw new UploadLimitError(
        "maxFileBytes",
        m.s1_upload_file_too_large({
          name: relativePath,
          mib: maxFileBytes >> 20,
        }),
        { name: file.name, relativePath },
      );
    }
  };

  async function walkEntry(
    entry: FileSystemEntry,
    parentPath: string,
  ): Promise<UploadNode | null> {
    checkAborted(signal);
    const name = entry.name;
    const relativePath = parentPath ? `${parentPath}/${name}` : name;
    if (skipGit && name.toLowerCase() === ".git") {
      skippedGit = true;
      return null;
    }
    if (isDirectoryEntry(entry)) {
      hadDirectory = true;
      accountEntry();
      directoryCount += 1;
      const rawChildren = await readAllDirectoryEntries(
        entry,
        relativePath,
        signal,
      );
      const children: UploadNode[] = [];
      for (const child of rawChildren) {
        const node = await walkEntry(child, relativePath);
        if (node) children.push(node);
      }
      return { kind: "directory", name, relativePath, children };
    }
    if (isFileEntry(entry)) {
      const file = await readEntryFile(entry, relativePath);
      checkAborted(signal);
      accountFile(file, relativePath);
      return { kind: "file", name, relativePath, file };
    }
    if (entry.isDirectory) {
      throw new Error(m.s1_upload_folder_unreadable({ name: relativePath }));
    }
    throw new Error(m.s1_upload_file_unreadable({ name: relativePath }));
  }

  const nodes: UploadNode[] = [];
  for (const item of source.items) {
    checkAborted(signal);
    if (item.kind === "file") {
      if (skipGit && item.file.name.toLowerCase() === ".git") {
        skippedGit = true;
        continue;
      }
      accountFile(item.file, item.file.name);
      nodes.push({
        kind: "file",
        name: item.file.name,
        relativePath: item.file.name,
        file: item.file,
      });
      continue;
    }
    const node = await walkEntry(item.entry, "");
    if (node) nodes.push(node);
  }

  return {
    nodes,
    fileCount,
    directoryCount,
    skippedGit,
    hadDirectory,
  };
}

/** Collects every file from an upload tree in depth-first order. */
export function flattenUploadFiles(nodes: readonly UploadNode[]): File[] {
  const files: File[] = [];
  const visit = (list: readonly UploadNode[]) => {
    for (const node of list) {
      if (node.kind === "file") files.push(node.file);
      else visit(node.children);
    }
  };
  visit(nodes);
  return files;
}
