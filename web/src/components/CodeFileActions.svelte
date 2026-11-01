<script lang="ts" module>
  import type { CodeNode } from "../schemas";
  /** A tree entry the menu acts on; the root has the empty path. */
  export type FileTarget = Pick<CodeNode, "nodeId" | "path" | "name" | "kind">;
</script>

<script lang="ts">
  import { errorMessage } from "../error";
  import type { z } from "zod";
  import { api, apiBlob, uploadForm } from "../api";
  import { saveBlob } from "../download";
  import { formatBytes, rawError } from "../code-image";
  import {
    CodeContextSchema,
    CodeCopyRequestSchema,
    CodeMkdirRequestSchema,
    CodeNameSchema,
    CodeMutationSchema,
    CodeRenameRequestSchema,
    CodeUsageSchema,
    NoContentSchema,
  } from "../schemas";
  import type { UploadProgress } from "../hooks/upload.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import FileUploadProgress from "./FileUploadProgress.svelte";
  import ResponsiveDialog from "./ResponsiveDialog.svelte";
  import UploadFileInput from "./UploadFileInput.svelte";
  import IconClipboard from "~icons/tabler/clipboard";
  import IconCopy from "~icons/tabler/copy";
  import IconDownload from "~icons/tabler/download";
  import IconEye from "~icons/tabler/eye";
  import IconFileZip from "~icons/tabler/file-zip";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconPencil from "~icons/tabler/pencil";
  import IconScissors from "~icons/tabler/scissors";
  import IconTrash from "~icons/tabler/trash";
  import IconUpload from "~icons/tabler/upload";

  type Clipboard = {
    contextId: string;
    nodes: FileTarget[];
    mode: "copy" | "move";
  };

  // The explorer's context menu: file operations on one tree entry, with the
  // copy/cut clipboard, dialogs and transfer progress they need.
  let {
    id,
    context,
    includeIgnored,
    openFile,
    changed,
  }: {
    id: string;
    context: z.infer<typeof CodeContextSchema> | undefined;
    includeIgnored: boolean;
    openFile: (path: string, nodeId: string) => void;
    /** After a change; removed lists paths that no longer exist. */
    changed: (removed: string[]) => void;
  } = $props();

  const base = $derived(`/sessions/${id}/code`);
  const writable = $derived(!!context?.writableAvailable);
  let menuOpen = $state(false);
  let target = $state<FileTarget>();
  let anchor = $state<HTMLElement | { getBoundingClientRect(): DOMRect }>();
  let returnFocus: HTMLElement | undefined;
  let clipboard = $state<Clipboard>();
  let notice = $state("");
  let error = $state("");
  let naming = $state<{ kind: "rename" | "mkdir"; node: FileTarget }>();
  let name = $state("");
  let busy = $state(false);
  let dialogError = $state("");
  let removing = $state<{ node: FileTarget; detail: string }>();
  let confirmOpen = $state(false);
  let picker = $state<UploadFileInput>();
  let uploadTarget: FileTarget | undefined;
  let progress = $state<UploadProgress | null>(null);
  let uploadController: AbortController | undefined;
  let download = $state<{ name: string; bytes: number }>();
  let downloadController: AbortController | undefined;
  const pasteReady = $derived(
    !!clipboard && clipboard.contextId === context?.contextId,
  );

  /**
   * Opens the menu for node, anchored at the control that asked or at the
   * pointer position of a context-menu event; focus then returns to row.
   */
  export function show(
    node: FileTarget,
    at: HTMLElement | { x: number; y: number },
    row: HTMLElement,
  ) {
    target = node;
    returnFocus = row;
    anchor =
      at instanceof HTMLElement
        ? at
        : { getBoundingClientRect: () => new DOMRect(at.x, at.y, 0, 0) };
    menuOpen = true;
  }

  function query(node: FileTarget, extra: Record<string, string> = {}) {
    return new URLSearchParams({
      contextId: context?.contextId ?? "",
      nodeId: node.nodeId,
      ...extra,
    });
  }

  // Runs one operation, reporting its outcome in the strip above the tree.
  async function run(action: () => Promise<string | void>) {
    error = "";
    notice = "";
    try {
      notice = (await action()) ?? "";
    } catch (e) {
      error = errorMessage(e);
    }
  }

  // Drops clipboard entries at or below paths that no longer exist.
  function forget(paths: string[]) {
    if (!clipboard) return;
    const nodes = clipboard.nodes.filter(
      (n) => !paths.some((p) => n.path === p || n.path.startsWith(p + "/")),
    );
    clipboard = nodes.length ? { ...clipboard, nodes } : undefined;
  }

  function startNaming(kind: "rename" | "mkdir", node: FileTarget) {
    naming = { kind, node };
    name = kind === "rename" ? node.name : "";
    dialogError = "";
  }

  async function submitName() {
    if (!naming || !context) return;
    const { kind, node } = naming;
    const checked = CodeNameSchema.safeParse(name);
    if (!checked.success) {
      dialogError = checked.error.issues[0]?.message ?? "名称无效";
      return;
    }
    busy = true;
    dialogError = "";
    try {
      const result =
        kind === "rename"
          ? await api(
              `${base}/rename`,
              CodeMutationSchema,
              "POST",
              { contextId: context.contextId, nodeId: node.nodeId, name },
              undefined,
              CodeRenameRequestSchema,
            )
          : await api(
              `${base}/mkdir`,
              CodeMutationSchema,
              "POST",
              { contextId: context.contextId, parentNodeId: node.nodeId, name },
              undefined,
              CodeMkdirRequestSchema,
            );
      naming = undefined;
      error = "";
      const placed = result.nodes[0]?.name ?? name;
      notice = placed === name ? "" : `名称已被占用，已改为 ${placed}`;
      if (kind === "rename" && placed !== node.name) {
        forget([node.path]);
        changed([node.path]);
      } else changed([]);
    } catch (e) {
      dialogError = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function paste(into: FileTarget) {
    const current = clipboard;
    if (!current || !context) return;
    await run(async () => {
      const result = await api(
        `${base}/copy`,
        CodeMutationSchema,
        "POST",
        {
          contextId: context.contextId,
          nodeIds: current.nodes.map((n) => n.nodeId),
          targetNodeId: into.nodeId,
          mode: current.mode,
        },
        undefined,
        CodeCopyRequestSchema,
      );
      if (current.mode === "move") {
        clipboard = undefined;
        changed(
          current.nodes
            .filter((n, i) => result.nodes[i]?.path !== n.path)
            .map((n) => n.path),
        );
      } else changed([]);
      const renamed = result.nodes.filter(
        (n, i) => n.name !== current.nodes[i]?.name,
      );
      return renamed.length
        ? `同名条目已存在，已粘贴为 ${renamed.map((n) => n.name).join("、")}`
        : `已${current.mode === "move" ? "移动" : "复制"} ${result.nodes.length} 项`;
    });
  }

  async function askRemove(node: FileTarget) {
    await run(async () => {
      let detail = `将删除${node.kind === "symlink" ? "符号链接（不影响其目标）" : "文件"} ${node.path}。此操作不能撤销。`;
      if (node.kind === "directory") {
        const usage = await api(
          `${base}/usage?${query(node)}`,
          CodeUsageSchema,
        );
        detail = `将递归删除文件夹 ${node.path} 及其中 ${usage.entries - 1}${usage.limited ? "+" : ""} 个条目（${formatBytes(usage.bytes)}）。此操作不能撤销。`;
      }
      removing = { node, detail };
      confirmOpen = true;
    });
  }

  async function remove() {
    const node = removing?.node;
    confirmOpen = false;
    if (!node) return;
    await run(async () => {
      await api(
        `${base}/node?${query(node, node.kind === "directory" ? { recursive: "1" } : {})}`,
        NoContentSchema,
        "DELETE",
      );
      forget([node.path]);
      changed([node.path]);
      return `已删除 ${node.name}`;
    });
  }

  async function upload(files: File[]) {
    const into = uploadTarget;
    if (!into || !files.length) return;
    uploadController?.abort();
    const controller = new AbortController();
    uploadController = controller;
    const stored: string[] = [];
    await run(async () => {
      try {
        for (const [index, file] of files.entries()) {
          const current = {
            name: file.name,
            percent: 0,
            index: index + 1,
            total: files.length,
            waiting: false,
          };
          progress = current;
          const result = await uploadForm(
            `${base}/upload?${query(into)}`,
            file,
            CodeMutationSchema,
            (percent) => (progress = { ...current, percent }),
            controller.signal,
          );
          stored.push(...result.nodes.map((n) => n.name));
        }
        return `已上传 ${stored.join("、")}`;
      } finally {
        if (uploadController === controller) {
          uploadController = undefined;
          progress = null;
        }
        if (stored.length) changed([]);
      }
    });
  }

  async function save(node: FileTarget, archive: boolean) {
    downloadController?.abort();
    const controller = new AbortController();
    downloadController = controller;
    const filename = archive ? `${node.name}.zip` : node.name;
    download = { name: filename, bytes: 0 };
    error = "";
    notice = "";
    try {
      const blob = await apiBlob(
        archive
          ? `${base}/archive?${query(node, { includeIgnored: String(includeIgnored) })}`
          : `${base}/node?${query(node)}`,
        controller.signal,
        (bytes) => {
          if (download) download.bytes = bytes;
        },
      );
      if (!controller.signal.aborted)
        saveBlob(returnFocus?.ownerDocument ?? document, blob, filename);
    } catch (e) {
      if (!controller.signal.aborted) error = rawError(e, "下载");
    } finally {
      if (downloadController === controller) {
        downloadController = undefined;
        download = undefined;
      }
    }
  }

  $effect(() => () => {
    uploadController?.abort();
    downloadController?.abort();
  });
</script>

<UploadFileInput bind:this={picker} {upload} label="选择上传到此目录的文件" />

{#if progress}<FileUploadProgress
    variant="card"
    {progress}
    cancel={() => uploadController?.abort()}
  />{/if}
{#if download}<div
    class="flex shrink-0 items-center gap-2 px-2 py-1 text-xs"
    role="status"
  >
    <span class="min-w-0 flex-1 truncate"
      >正在{download.name.endsWith(".zip") ? "压缩并" : ""}下载 {download.name}
      · {formatBytes(download.bytes)}</span
    >
    <Button
      variant="ghost"
      size="xs"
      onclick={() => downloadController?.abort()}>取消</Button
    >
  </div>{/if}
{#if error}<p
    role="alert"
    class="shrink-0 px-2 py-1 text-xs break-all text-destructive"
  >
    {error}
  </p>{:else if notice}<p
    role="status"
    class="shrink-0 px-2 py-1 text-xs break-all text-muted-foreground"
  >
    {notice}
  </p>{/if}

<DropdownMenu.Root bind:open={menuOpen}>
  <DropdownMenu.Content
    customAnchor={anchor}
    align="start"
    class="w-56"
    aria-label={target ? `文件操作 ${target.name}` : "文件操作"}
    onCloseAutoFocus={(event) => {
      event.preventDefault();
      returnFocus?.focus();
    }}
  >
    {#if target}
      {@const node = target}
      {@const folder = node.kind === "directory"}
      {@const editable =
        node.path !== "" && (node.kind === "file" || node.kind === "directory")}
      {#if node.kind === "file"}
        <DropdownMenu.Item onSelect={() => openFile(node.path, node.nodeId)}
          ><IconEye aria-hidden="true" />打开</DropdownMenu.Item
        >
        <DropdownMenu.Item onSelect={() => void save(node, false)}
          ><IconDownload aria-hidden="true" />下载</DropdownMenu.Item
        >
      {/if}
      {#if folder}
        <DropdownMenu.Item onSelect={() => void save(node, true)}
          ><IconFileZip aria-hidden="true" />下载为 ZIP</DropdownMenu.Item
        >
        <DropdownMenu.Separator />
        <DropdownMenu.Item
          disabled={!writable}
          onSelect={() => {
            uploadTarget = node;
            picker?.open();
          }}><IconUpload aria-hidden="true" />上传到此目录</DropdownMenu.Item
        >
        <DropdownMenu.Item
          disabled={!writable}
          onSelect={() => startNaming("mkdir", node)}
          ><IconFolderPlus aria-hidden="true" />新建文件夹</DropdownMenu.Item
        >
      {/if}
      {#if editable}
        <DropdownMenu.Separator />
        <DropdownMenu.Item
          disabled={!writable}
          onSelect={() => startNaming("rename", node)}
          ><IconPencil aria-hidden="true" />重命名</DropdownMenu.Item
        >
        <DropdownMenu.Item
          onSelect={() =>
            (clipboard = {
              contextId: context?.contextId ?? "",
              nodes: [node],
              mode: "copy",
            })}><IconCopy aria-hidden="true" />复制</DropdownMenu.Item
        >
        <DropdownMenu.Item
          disabled={!writable}
          onSelect={() =>
            (clipboard = {
              contextId: context?.contextId ?? "",
              nodes: [node],
              mode: "move",
            })}><IconScissors aria-hidden="true" />剪切</DropdownMenu.Item
        >
      {/if}
      {#if folder}
        <DropdownMenu.Item
          disabled={!writable || !pasteReady}
          onSelect={() => void paste(node)}
          ><IconClipboard aria-hidden="true" /><span class="truncate"
            >粘贴{pasteReady && clipboard
              ? `（${clipboard.mode === "move" ? "移动" : "复制"} ${clipboard.nodes[0]?.name ?? ""}）`
              : ""}</span
          ></DropdownMenu.Item
        >
      {/if}
      {#if node.kind === "boundary" || node.kind === "special"}
        <DropdownMenu.Item disabled>此类条目不支持文件操作</DropdownMenu.Item>
      {:else if node.path !== ""}
        <DropdownMenu.Separator />
        <DropdownMenu.Item
          variant="destructive"
          disabled={!writable}
          onSelect={() => void askRemove(node)}
          ><IconTrash aria-hidden="true" />删除…</DropdownMenu.Item
        >
      {/if}
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>

{#if naming}
  {@const { kind, node } = naming}
  <ResponsiveDialog
    title={kind === "rename" ? "重命名" : "新建文件夹"}
    description={kind === "rename"
      ? `在原目录中重命名 ${node.path}；名称已被占用时自动追加 (n)。`
      : `在 ${node.path || "根目录"} 中新建文件夹；名称已被占用时自动追加 (n)。`}
    onclose={() => (naming = undefined)}
    {busy}
  >
    <form
      class="space-y-4"
      onsubmit={(event) => {
        event.preventDefault();
        void submitName();
      }}
    >
      <div class="space-y-2">
        <Label for="code-file-name">名称</Label>
        <Input
          id="code-file-name"
          bind:value={name}
          required
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
      </div>
      {#if dialogError}<p
          role="alert"
          class="text-sm break-all text-destructive"
        >
          {dialogError}
        </p>{/if}
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onclick={() => (naming = undefined)}>取消</Button
        >
        <Button type="submit" disabled={busy || !name}
          >{kind === "rename" ? "重命名" : "新建"}</Button
        >
      </div>
    </form>
  </ResponsiveDialog>
{/if}

<ConfirmDialog
  bind:open={confirmOpen}
  title={`删除 ${removing?.node.name ?? ""}`}
  onconfirm={() => void remove()}
>
  {removing?.detail ?? ""}
</ConfirmDialog>
