<script lang="ts" module>
  export type { FileTarget } from "../../domains/code/code-file-actions";
</script>

<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import { untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import type { z } from "zod";
  import {
    buildCodeFileActions,
    cloneClipboard,
    describeCodeRemoveDetail,
    downloadCodeNode,
    dropClipboardPaths,
    formatClipboardSummary,
    formatDownloadProgress,
    makeRootFileTarget,
    requestCodePaste,
    requestCodeRemove,
    runNamingMutation,
    selectFileStem,
    toMenuAnchor,
    type CodeFileClipboard,
    type FileTarget,
  } from "../../domains/code/code-file-actions";
  import { uploadCodeEntriesBatch } from "../../domains/code/code-upload";
  import {
    filesToDroppedSource,
    type DroppedSource,
  } from "../../platform/upload-entries";
  import { CodeContextSchema } from "../../contracts/schemas";
  import type { UploadProgress } from "../../hooks/shared/upload.svelte";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as Drawer from "$lib/components/ui/drawer";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import CodeFileNameDialog from "./CodeFileNameDialog.svelte";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import FileUploadProgress from "../shared/FileUploadProgress.svelte";
  import UploadFileInput from "../shared/UploadFileInput.svelte";
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

  type Clipboard = CodeFileClipboard;

  interface Props {
    id: string;
    context: z.infer<typeof CodeContextSchema> | undefined;
    includeIgnored: boolean;
    openFile: (path: string, nodeId: string) => void;
    /** After a change; removed lists paths that no longer exist. */
    changed: (removed: string[]) => void;
    /** The copy/cut clipboard to start with. */
    restore?: Clipboard;
  }

  let { id, context, includeIgnored, openFile, changed, restore }: Props =
    $props();

  const base = $derived(`/sessions/${id}/code`);
  const writable = $derived(!!context?.writableAvailable);
  let menuOpen = $state(false);
  let target = $state<FileTarget>();
  let anchor = $state<HTMLElement | { getBoundingClientRect(): DOMRect }>();
  let returnFocus: HTMLElement | undefined;
  let clipboard = $state<Clipboard | undefined>(untrack(() => restore));
  let notice = $state("");
  let error = $state("");
  let naming = $state<{ kind: "rename" | "mkdir"; node: FileTarget }>();
  let name = $state("");
  let busy = $state(false);
  let dialogError = $state("");
  let removing = $state<{ node: FileTarget; detail: string }>();
  let confirmOpen = $state(false);
  let picker = $state<UploadFileInput>();
  let pickerTarget: FileTarget | undefined;
  let progress = $state<UploadProgress | null>(null);
  let uploadController: AbortController | undefined;
  let download = $state<{ name: string; bytes: number }>();
  let downloadController: AbortController | undefined;
  const pasteReady = $derived(
    !!clipboard && clipboard.contextId === context?.contextId,
  );
  const desktop = new MediaQuery("(min-width: 640px)");
  let nameInput = $state<HTMLInputElement | null>(null);
  const rootTarget = $derived(makeRootFileTarget(context?.rootPath));
  const actions = $derived(
    buildCodeFileActions({
      node: target,
      writable,
      pasteReady,
      clipboard,
      contextId: context?.contextId ?? "",
      icons: {
        eye: IconEye,
        download: IconDownload,
        fileZip: IconFileZip,
        upload: IconUpload,
        folderPlus: IconFolderPlus,
        pencil: IconPencil,
        copy: IconCopy,
        scissors: IconScissors,
        clipboard: IconClipboard,
        trash: IconTrash,
      },
      openFile,
      save: (node, archive) => void save(node, archive),
      openUploadPicker: (node) => {
        pickerTarget = node;
        picker?.open();
      },
      startNaming,
      setClipboard: (next) => (clipboard = next),
      paste: (node) => void paste(node),
      askRemove: (node) => void askRemove(node),
    }),
  );
  $effect(() => {
    const input = nameInput;
    const current = naming;
    if (!input || !current) return;
    untrack(() => selectFileStem(input, current.kind, current.node.kind, name));
  });

  export function show(
    node: FileTarget,
    at: HTMLElement | { x: number; y: number },
    row: HTMLElement,
  ) {
    target = node;
    returnFocus = row;
    anchor = toMenuAnchor(at);
    menuOpen = true;
  }

  function restoreRowFocus(event: Event) {
    event.preventDefault();
    if (!menuOpen && !naming && !confirmOpen)
      returnFocus?.focus({ preventScroll: true });
  }

  export function capture(): Clipboard | undefined {
    return cloneClipboard(clipboard);
  }

  export function uploadRefusal(): string {
    if (!writable) return m.s5_readonly_dir_no_upload();
    return uploadController ? m.s5_wait_current_upload() : "";
  }

  export function refuse(message: string) {
    notice = "";
    error = message;
  }

  async function run(action: () => Promise<string | void>) {
    error = "";
    notice = "";
    try {
      const outcome = (await action()) ?? "";
      error = "";
      notice = outcome;
    } catch (e) {
      notice = "";
      error = errorMessage(e);
    }
  }

  function forget(paths: string[]) {
    clipboard = dropClipboardPaths(clipboard, paths);
  }

  function startNaming(kind: "rename" | "mkdir", node: FileTarget) {
    naming = { kind, node };
    name = kind === "rename" ? node.name : "";
    dialogError = "";
  }

  async function submitName() {
    if (!naming || !context) return;
    busy = true;
    dialogError = "";
    try {
      const result = await runNamingMutation(
        base,
        context.contextId,
        naming.kind,
        naming.node,
        name,
        changed,
        forget,
      );
      if (result.error) {
        dialogError = result.error;
        return;
      }
      naming = undefined;
      error = "";
      notice = result.notice ?? "";
    } catch (e) {
      dialogError = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function paste(into: FileTarget) {
    const current = clipboard;
    if (!current || !context) return;
    await run(() =>
      requestCodePaste(
        base,
        context.contextId,
        current,
        into,
        changed,
        () => (clipboard = undefined),
      ),
    );
  }

  async function askRemove(node: FileTarget) {
    await run(async () => {
      const detail = await describeCodeRemoveDetail(
        base,
        context?.contextId ?? "",
        node,
      );
      removing = { node, detail };
      confirmOpen = true;
    });
  }

  async function remove() {
    const node = removing?.node;
    confirmOpen = false;
    if (!node) return;
    await run(() =>
      requestCodeRemove(base, context?.contextId ?? "", node, forget, changed),
    );
  }

  export async function uploadSource(source: DroppedSource, into: FileTarget) {
    if (!source.items.length) return "";
    uploadController?.abort();
    const controller = new AbortController();
    uploadController = controller;
    await run(() =>
      uploadCodeEntriesBatch({
        base,
        contextId: context?.contextId ?? "",
        into,
        source,
        signal: controller.signal,
        onProgress: (next) => (progress = next),
        onCleanup: (storedCount) => {
          if (uploadController === controller) {
            uploadController = undefined;
            progress = null;
          }
          if (storedCount) changed([]);
        },
      }),
    );
    return error || notice;
  }

  export async function upload(files: File[], into: FileTarget) {
    return uploadSource(filesToDroppedSource(files), into);
  }

  async function save(node: FileTarget, archive: boolean) {
    downloadController?.abort();
    const controller = new AbortController();
    downloadController = controller;
    error = "";
    notice = "";
    await downloadCodeNode({
      base,
      contextId: context?.contextId ?? "",
      node,
      archive,
      includeIgnored,
      signal: controller.signal,
      ownerDocument: returnFocus?.ownerDocument ?? document,
      onStart: (name) => (download = { name, bytes: 0 }),
      onProgress: (bytes) => {
        if (download) download.bytes = bytes;
      },
      onError: (msg) => (error = msg),
      onDone: () => {
        if (downloadController === controller) {
          downloadController = undefined;
          download = undefined;
        }
      },
    });
  }

  $effect(() => () => {
    uploadController?.abort();
    downloadController?.abort();
  });
</script>

<UploadFileInput
  bind:this={picker}
  upload={async (files) => {
    if (pickerTarget) await upload(files, pickerTarget);
  }}
  label={m.s5_select_files_to_upload()}
/>

{#if progress}<FileUploadProgress
    variant="card"
    {progress}
    cancel={() => uploadController?.abort()}
  />{/if}
{#if download}
  {@const status = formatDownloadProgress(download.name, download.bytes)}
  <div class="flex shrink-0 items-center gap-2 px-2 py-1 text-xs" role="status">
    <span class="min-w-0 flex-1 truncate" title={download.name}
      >{status.label}</span
    >
    <span class="shrink-0 tabular-nums">{status.size}</span>
    <Button
      variant="ghost"
      size="xs"
      onclick={() => downloadController?.abort()}>{m.s5_cancel()}</Button
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

{#if pasteReady && clipboard}<div
    class="flex shrink-0 items-center gap-1 border-y bg-muted/50 px-2 py-1 text-xs"
    role="status"
    aria-label={m.s5_clipboard()}
  >
    <span
      class="min-w-0 flex-1 truncate"
      title={m.s5_paste_in_folder_menu_hint()}
      >{formatClipboardSummary(clipboard)}</span
    >
    <Button
      variant="ghost"
      size="xs"
      disabled={!writable}
      onclick={() => void paste(rootTarget)}>{m.s5_paste_to_root()}</Button
    >
    <Button variant="ghost" size="xs" onclick={() => (clipboard = undefined)}
      >{m.s5_clear()}</Button
    >
  </div>{/if}

{#if desktop.current}
  <DropdownMenu.Root bind:open={menuOpen}>
    <DropdownMenu.Content
      customAnchor={anchor}
      align="start"
      class="w-56"
      aria-label={target
        ? m.s5_file_actions_for({ name: target.name })
        : m.s5_file_actions()}
      onCloseAutoFocus={restoreRowFocus}
    >
      {#each actions as action, index (action.label)}
        {#if index > 0 && action.group !== actions[index - 1]?.group}<DropdownMenu.Separator
          />{/if}
        <DropdownMenu.Item
          variant={action.destructive ? "destructive" : "default"}
          disabled={action.disabled}
          onSelect={action.run}
          >{#if action.icon}<action.icon aria-hidden="true" />{/if}<span
            class="truncate">{action.label}</span
          ></DropdownMenu.Item
        >
      {/each}
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{:else}
  <Drawer.Root bind:open={menuOpen}>
    <Drawer.Content
      onCloseAutoFocus={restoreRowFocus}
      aria-label={target
        ? m.s5_file_actions_for({ name: target.name })
        : m.s5_file_actions()}
    >
      <Drawer.Header>
        <Drawer.Title>{target?.name ?? ""}</Drawer.Title>
        <Drawer.Description
          >{target?.path
            ? `${context?.rootPath ?? ""}/${target.path}`
            : (context?.rootPath ?? "")}</Drawer.Description
        >
      </Drawer.Header>
      <div class="flex min-h-0 flex-col overflow-y-auto px-2 pb-safe-4">
        {#each actions as action, index (action.label)}
          {#if index > 0 && action.group !== actions[index - 1]?.group}<div
              class="px-3 py-1"
            >
              <Separator />
            </div>{/if}
          <ListButton
            variant="action-sheet"
            data-destructive={action.destructive}
            disabled={action.disabled}
            onclick={() => {
              menuOpen = false;
              action.run();
            }}
            >{#if action.icon}<action.icon aria-hidden="true" />{/if}<span
              class="min-w-0 truncate">{action.label}</span
            ></ListButton
          >
        {/each}
        <Button
          variant="outline"
          class="mt-2 w-full"
          onclick={() => (menuOpen = false)}>{m.s5_cancel()}</Button
        >
      </div>
    </Drawer.Content>
  </Drawer.Root>
{/if}

{#if naming}
  <CodeFileNameDialog
    kind={naming.kind}
    node={naming.node}
    bind:name
    bind:nameInput
    {busy}
    {dialogError}
    onsubmit={() => void submitName()}
    onclose={() => (naming = undefined)}
  />
{/if}

<ConfirmDialog
  bind:open={confirmOpen}
  title={m.s5_delete_target_title({ name: removing?.node.name ?? "" })}
  onconfirm={() => void remove()}
>
  {removing?.detail ?? ""}
</ConfirmDialog>
