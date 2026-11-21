import type { Component } from "svelte";
import { m } from "$lib/paraglide/messages.js";
import { api, apiBlob } from "../../platform/api";
import { saveBlob } from "../../platform/download";
import { formatBytes, rawError } from "./code-image";
import {
  CodeCopyRequestSchema,
  CodeMkdirRequestSchema,
  CodeMutationSchema,
  CodeNameSchema,
  CodeRenameRequestSchema,
  CodeUsageSchema,
  NoContentSchema,
  type CodeNavigatorState,
  type CodeNode,
} from "../../contracts/schemas";

/** A tree entry the menu acts on; the root has the empty path. */
export type FileTarget = Pick<CodeNode, "nodeId" | "path" | "name" | "kind">;

export type CodeFileClipboard = NonNullable<CodeNavigatorState["clipboard"]>;

export type CodeFileActionItem = {
  label: string;
  icon?: Component;
  run: () => void;
  disabled?: boolean;
  destructive?: boolean;
  // Actions of one group sit between separators.
  group: number;
};

export function makeRootFileTarget(rootPath: string | undefined): FileTarget {
  return {
    nodeId: "",
    path: "",
    name: rootPath?.split("/").filter(Boolean).at(-1) ?? m.s5_root_directory(),
    kind: "directory",
  };
}

export function buildCodeFileActions(input: {
  node: FileTarget | undefined;
  writable: boolean;
  pasteReady: boolean;
  clipboard: CodeFileClipboard | undefined;
  contextId: string;
  icons: {
    eye: Component;
    download: Component;
    fileZip: Component;
    upload: Component;
    folderPlus: Component;
    pencil: Component;
    copy: Component;
    scissors: Component;
    clipboard: Component;
    trash: Component;
  };
  openFile: (path: string, nodeId: string) => void;
  openRight?: (path: string, nodeId: string) => void;
  save: (node: FileTarget, archive: boolean) => void;
  openUploadPicker: (node: FileTarget) => void;
  startNaming: (kind: "rename" | "mkdir", node: FileTarget) => void;
  setClipboard: (clipboard: CodeFileClipboard) => void;
  paste: (node: FileTarget) => void;
  askRemove: (node: FileTarget) => void;
}): CodeFileActionItem[] {
  const { node, writable, pasteReady, clipboard, icons } = input;
  if (!node) return [];
  if (node.kind === "boundary" || node.kind === "special")
    return [
      {
        label: m.s5_actions_unsupported_entry(),
        disabled: true,
        group: 0,
        run() {},
      },
    ];
  const folder = node.kind === "directory";
  const editable = node.path !== "" && (node.kind === "file" || folder);
  const hold = (mode: CodeFileClipboard["mode"]) => () => {
    input.setClipboard({
      contextId: input.contextId,
      nodes: [node],
      mode,
    });
  };
  const list: CodeFileActionItem[] = [];
  if (node.kind === "file")
    list.push(
      {
        label: m.s5_open(),
        icon: icons.eye,
        group: 0,
        run: () => input.openFile(node.path, node.nodeId),
      },
      ...(input.openRight
        ? [
            {
              label: m.s5_open_right(),
              group: 0,
              run: () => input.openRight?.(node.path, node.nodeId),
            },
          ]
        : []),
      {
        label: m.s5_action_download(),
        icon: icons.download,
        group: 0,
        run: () => input.save(node, false),
      },
    );
  if (folder)
    list.push(
      {
        label: m.s5_download_as_zip(),
        icon: icons.fileZip,
        group: 0,
        run: () => input.save(node, true),
      },
      {
        label: m.s5_upload_to_dir(),
        icon: icons.upload,
        group: 1,
        disabled: !writable,
        run: () => input.openUploadPicker(node),
      },
      {
        label: m.s5_new_folder(),
        icon: icons.folderPlus,
        group: 1,
        disabled: !writable,
        run: () => input.startNaming("mkdir", node),
      },
    );
  if (editable)
    list.push(
      {
        label: m.s5_rename(),
        icon: icons.pencil,
        group: 2,
        disabled: !writable,
        run: () => input.startNaming("rename", node),
      },
      { label: m.s5_copy(), icon: icons.copy, group: 2, run: hold("copy") },
      {
        label: m.s5_cut(),
        icon: icons.scissors,
        group: 2,
        disabled: !writable,
        run: hold("move"),
      },
    );
  if (folder)
    list.push({
      label:
        pasteReady && clipboard
          ? clipboard.mode === "move"
            ? m.s5_paste_move_item({ name: clipboard.nodes[0]?.name ?? "" })
            : m.s5_paste_copy_item({ name: clipboard.nodes[0]?.name ?? "" })
          : m.s5_paste(),
      icon: icons.clipboard,
      group: 2,
      disabled: !writable || !pasteReady,
      run: () => input.paste(node),
    });
  if (node.path !== "")
    list.push({
      label: m.s5_delete_ellipsis(),
      icon: icons.trash,
      group: 3,
      destructive: true,
      disabled: !writable,
      run: () => input.askRemove(node),
    });
  return list;
}

function buildCodeNodeQuery(
  contextId: string,
  node: FileTarget,
  extra: Record<string, string> = {},
) {
  return new URLSearchParams({
    contextId,
    nodeId: node.nodeId,
    ...extra,
  });
}

export function toMenuAnchor(at: HTMLElement | { x: number; y: number }) {
  return at instanceof HTMLElement
    ? at
    : { getBoundingClientRect: () => new DOMRect(at.x, at.y, 0, 0) };
}

export async function runNamingMutation(
  base: string,
  contextId: string,
  kind: "rename" | "mkdir",
  node: FileTarget,
  name: string,
  changed: (removed: string[]) => void,
  forget: (paths: string[]) => void,
): Promise<{ notice?: string; error?: string }> {
  const checked = CodeNameSchema.safeParse(name);
  if (!checked.success) {
    return { error: checked.error.issues[0]?.message ?? m.s5_invalid_name() };
  }
  const result =
    kind === "rename"
      ? await api(
          `${base}/rename`,
          CodeMutationSchema,
          "POST",
          { contextId, nodeId: node.nodeId, name },
          undefined,
          CodeRenameRequestSchema,
        )
      : await api(
          `${base}/mkdir`,
          CodeMutationSchema,
          "POST",
          { contextId, parentNodeId: node.nodeId, name },
          undefined,
          CodeMkdirRequestSchema,
        );
  const placed = result.nodes[0]?.name ?? name;
  if (kind === "rename" && placed !== node.name) {
    forget([node.path]);
    changed([node.path]);
  } else changed([]);
  return {
    notice: placed === name ? "" : m.s5_name_taken_renamed({ name: placed }),
  };
}

export async function requestCodePaste(
  base: string,
  contextId: string,
  current: CodeFileClipboard,
  into: FileTarget,
  changed: (removed: string[]) => void,
  clearClipboard: () => void,
): Promise<string> {
  const result = await api(
    `${base}/copy`,
    CodeMutationSchema,
    "POST",
    {
      contextId,
      nodeIds: current.nodes.map((n) => n.nodeId),
      targetNodeId: into.nodeId,
      mode: current.mode,
    },
    undefined,
    CodeCopyRequestSchema,
  );
  if (current.mode === "move") {
    clearClipboard();
    changed(
      current.nodes
        .filter((n, i) => result.nodes[i]?.path !== n.path)
        .map((n) => n.path),
    );
  } else changed([]);
  const renamed = result.nodes.filter(
    (n, i) => n.name !== current.nodes[i]?.name,
  );
  if (renamed.length) {
    return m.s5_pasted_with_renames({
      names: renamed.map((n) => n.name).join(m.s5_list_separator()),
    });
  }
  const count = String(result.nodes.length);
  return current.mode === "move"
    ? m.s5_moved_items_count({ count })
    : m.s5_copied_items_count({ count });
}

export async function describeCodeRemoveDetail(
  base: string,
  contextId: string,
  node: FileTarget,
): Promise<string> {
  if (node.kind !== "directory")
    return node.kind === "symlink"
      ? m.s5_confirm_delete_symlink({ path: node.path })
      : m.s5_confirm_delete_file({ path: node.path });
  const usage = await api(
    `${base}/usage?${buildCodeNodeQuery(contextId, node)}`,
    CodeUsageSchema,
  );
  return m.s5_confirm_delete_folder({
    path: node.path,
    count: `${usage.entries - 1}${usage.limited ? "+" : ""}`,
    size: formatBytes(usage.bytes),
  });
}

export async function requestCodeRemove(
  base: string,
  contextId: string,
  node: FileTarget,
  forget: (paths: string[]) => void,
  changed: (removed: string[]) => void,
): Promise<string> {
  await api(
    `${base}/node?${buildCodeNodeQuery(contextId, node, node.kind === "directory" ? { recursive: "1" } : {})}`,
    NoContentSchema,
    "DELETE",
  );
  forget([node.path]);
  changed([node.path]);
  return m.s5_deleted_node({ name: node.name });
}

export async function downloadCodeNode(input: {
  base: string;
  contextId: string;
  node: FileTarget;
  archive: boolean;
  includeIgnored: boolean;
  signal: AbortSignal;
  ownerDocument: Document;
  onStart: (filename: string) => void;
  onProgress: (bytes: number) => void;
  onError: (message: string) => void;
  onDone: () => void;
}) {
  const filename = input.archive ? `${input.node.name}.zip` : input.node.name;
  input.onStart(filename);
  try {
    const blob = await apiBlob(
      input.archive
        ? `${input.base}/archive?${buildCodeNodeQuery(input.contextId, input.node, { includeIgnored: String(input.includeIgnored) })}`
        : `${input.base}/node?${buildCodeNodeQuery(input.contextId, input.node)}`,
      input.signal,
      input.onProgress,
    );
    if (!input.signal.aborted) saveBlob(input.ownerDocument, blob, filename);
  } catch (e) {
    if (!input.signal.aborted) input.onError(rawError(e, "download"));
  } finally {
    input.onDone();
  }
}

export function formatDownloadProgress(name: string, bytes: number) {
  return {
    label: name.endsWith(".zip")
      ? m.s5_archiving_and_downloading({ name })
      : m.s5_downloading_progress({ name }),
    size: formatBytes(bytes),
  };
}

export function formatClipboardSummary(clipboard: CodeFileClipboard) {
  const items = clipboard.nodes.map((n) => n.name).join(m.s5_list_separator());
  return clipboard.mode === "move"
    ? m.s5_clipboard_cut_items({ items })
    : m.s5_clipboard_copied_items({ items });
}

export function namingDialogDescription(
  kind: "rename" | "mkdir",
  node: FileTarget,
) {
  return kind === "rename"
    ? m.s5_rename_dialog_desc({ path: node.path })
    : m.s5_mkdir_dialog_desc({ path: node.path || m.s5_root_directory() });
}

export function selectFileStem(
  input: HTMLInputElement,
  kind: "rename" | "mkdir",
  nodeKind: string,
  name: string,
) {
  const dot =
    kind === "rename" && nodeKind === "file" ? name.lastIndexOf(".") : -1;
  input.focus();
  input.setSelectionRange(0, dot > 0 ? dot : name.length);
}

export function dropClipboardPaths(
  clipboard: CodeFileClipboard | undefined,
  paths: string[],
): CodeFileClipboard | undefined {
  if (!clipboard) return undefined;
  const nodes = clipboard.nodes.filter(
    (n) => !paths.some((p) => n.path === p || n.path.startsWith(p + "/")),
  );
  return nodes.length ? { ...clipboard, nodes } : undefined;
}

export function cloneClipboard(
  clipboard: CodeFileClipboard | undefined,
): CodeFileClipboard | undefined {
  if (!clipboard) return undefined;
  const { contextId, nodes, mode } = clipboard;
  return {
    contextId,
    mode,
    nodes: nodes.map(({ nodeId, path, name, kind }) => ({
      nodeId,
      path,
      name,
      kind,
    })),
  };
}
