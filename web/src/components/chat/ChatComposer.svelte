<script lang="ts">
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconFile from "~icons/tabler/file";
  import IconMessageBolt from "~icons/tabler/message-bolt";
  import IconPaperclip from "~icons/tabler/paperclip";
  import IconPlayerStopFilled from "~icons/tabler/player-stop-filled";
  import IconPhoto from "~icons/tabler/photo";
  import IconX from "~icons/tabler/x";
  import { onDestroy } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { ChatActions } from "../../chat-api";
  import { errorMessage } from "../../error";
  import type {
    ChatAttachment,
    ChatCommand,
    ChatPermissionMode as PermissionMode,
    ChatState,
  } from "../../schemas";
  import type {
    TerminalDraft,
    TerminalDrafts,
  } from "../../terminal-draft.svelte";
  import { hasTouchInput } from "../../touchInput";
  import ChatConfig from "./ChatConfig.svelte";
  import ChatPermissionMode from "./ChatPermissionMode.svelte";
  import UsageRing from "./UsageRing.svelte";

  // The chat input. Its text is the session's draft, saved on this device
  // like the terminal draft. Sending while a turn runs queues on the server;
  // "插话" appears only when the agent can take text into the running turn.
  // "/" opens the agent's commands.
  let {
    draft,
    drafts,
    chat,
    unavailable,
    actions,
    onerror,
  }: {
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    chat: ChatState;
    /** Why input is disabled now (switching, offline); "" when enabled. */
    unavailable: string;
    actions: ChatActions;
    onerror: (message: string) => void;
  } = $props();

  const maxAttachments = 8;
  const maxAttachmentBytes = 10 * 1024 * 1024;
  const maxAttachmentTotalBytes = 20 * 1024 * 1024;
  // The kinds the server sends as native image blocks.
  const imageTypes = new Set([
    "image/png",
    "image/jpeg",
    "image/gif",
    "image/webp",
  ]);

  let textarea = $state<HTMLTextAreaElement | null>(null);
  let fileInput = $state<HTMLInputElement | null>(null);
  let sending = $state(false);
  let attachments = $state<ChatAttachment[]>([]);
  let uploading = $state<{
    name: string;
    index: number;
    total: number;
    percent: number;
  } | null>(null);
  let uploadController: AbortController | undefined;
  let dragActive = $state(false);
  onDestroy(() => uploadController?.abort());
  // A permission mode change is on its way; the select waits for it.
  let savingMode = $state(false);
  let highlighted = $state(0);
  // Escape hides the menu for the text it was open on.
  let menuDismissed = $state<string | null>(null);
  const running = $derived(chat.turn.running);
  const typed = $derived(draft.text.trim() !== "");
  const ready = $derived(typed || attachments.length > 0);
  const locked = $derived(!!draft.submission);
  const disabled = $derived(!!unavailable || locked);
  const steerable = $derived(running && chat.steering);
  const queueing = $derived(running || chat.queue.paused);
  const imageDegraded = $derived(
    !chat.promptCapabilities.image &&
      attachments.some((item) => item.kind === "image"),
  );
  const query = $derived(
    /^\/\S*$/.test(draft.text) ? draft.text.slice(1).toLowerCase() : null,
  );
  const matches = $derived(
    query === null
      ? []
      : chat.commands.filter((command) =>
          command.name.toLowerCase().startsWith(query),
        ),
  );
  const menuOpen = $derived(
    !disabled && matches.length > 0 && menuDismissed !== draft.text,
  );
  const active = $derived(Math.min(highlighted, matches.length - 1));

  async function submit(mode: "auto" | "steer") {
    const text = draft.text.trim();
    if ((!text && attachments.length === 0) || sending || disabled || uploading)
      return;
    const sent = attachments;
    sending = true;
    drafts.edit(draft, "", 0);
    attachments = [];
    try {
      await actions.send(text, mode, sent);
      onerror("");
    } catch (err) {
      // Give the text and attachments back unless the user typed something new.
      if (!draft.text) drafts.edit(draft, text, text.length);
      attachments = [...sent, ...attachments];
      onerror(errorMessage(err));
    } finally {
      sending = false;
    }
  }

  function pickFiles(files: FileList | null | undefined) {
    if (!files?.length) return;
    void uploadFiles(Array.from(files));
  }
  async function uploadFiles(files: File[]) {
    if (!files.length || uploading || disabled) return;
    if (attachments.length + files.length > maxAttachments) {
      onerror(`每条消息最多 ${maxAttachments} 个附件。`);
      return;
    }
    const oversized = files.find((file) => file.size > maxAttachmentBytes);
    if (oversized) {
      onerror(`${oversized.name} 超过 10 MiB。`);
      return;
    }
    const total =
      attachments.reduce((sum, item) => sum + (item.size ?? 0), 0) +
      files.reduce((sum, file) => sum + file.size, 0);
    if (total > maxAttachmentTotalBytes) {
      onerror("附件合计超过 20 MiB。");
      return;
    }
    const controller = new AbortController();
    uploadController = controller;
    onerror("");
    try {
      for (const [index, file] of files.entries()) {
        uploading = {
          name: file.name,
          index: index + 1,
          total: files.length,
          percent: 0,
        };
        const { path } = await actions.upload(
          file,
          (percent) => {
            if (uploading) uploading.percent = percent;
          },
          controller.signal,
        );
        attachments = [
          ...attachments,
          {
            path,
            name: file.name,
            mimeType: file.type || undefined,
            size: file.size,
            kind: imageTypes.has(file.type) ? "image" : "file",
          },
        ];
      }
    } catch (err) {
      onerror(
        err instanceof DOMException && err.name === "AbortError"
          ? "已取消上传"
          : errorMessage(err),
      );
    } finally {
      uploading = null;
      uploadController = undefined;
    }
  }
  function removeAttachment(path: string) {
    attachments = attachments.filter((item) => item.path !== path);
  }
  function paste(event: ClipboardEvent) {
    const files = Array.from(event.clipboardData?.files ?? []);
    if (!files.length) return;
    event.preventDefault();
    void uploadFiles(files);
  }
  function drop(event: DragEvent) {
    if (!event.dataTransfer?.types.includes("Files")) return;
    event.preventDefault();
    dragActive = false;
    pickFiles(event.dataTransfer.files);
  }
  function pick(command: ChatCommand) {
    const text = `/${command.name} `;
    drafts.edit(draft, text, text.length);
    textarea?.focus();
  }
  function keydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (menuOpen) {
      const command = matches[active];
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : -1;
        highlighted = (active + step + matches.length) % matches.length;
        return;
      }
      if ((event.key === "Enter" || event.key === "Tab") && command) {
        event.preventDefault();
        pick(command);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        menuDismissed = draft.text;
        return;
      }
    }
    if (event.key !== "Enter" || event.shiftKey) return;
    const steer = event.ctrlKey || event.metaKey;
    // On touch keyboards Enter starts a new line; the button sends.
    if (!steer && hasTouchInput()) return;
    event.preventDefault();
    void submit(steer && steerable ? "steer" : "auto");
  }
  async function stop() {
    try {
      await actions.cancel();
    } catch (err) {
      onerror(errorMessage(err));
    }
  }
  async function setPermissionMode(mode: PermissionMode) {
    savingMode = true;
    try {
      await actions.setPermissionMode(mode);
      onerror("");
    } catch (err) {
      onerror(errorMessage(err));
    } finally {
      savingMode = false;
    }
  }
</script>

<div
  class={["space-y-1.5 rounded-xl", dragActive ? "ring-2 ring-ring/40" : ""]}
  data-chat-composer
  role="group"
  aria-label="消息输入"
  ondragover={(event) => {
    if (!event.dataTransfer?.types.includes("Files")) return;
    event.preventDefault();
    dragActive = true;
  }}
  ondragleave={() => (dragActive = false)}
  ondrop={drop}
>
  <!-- The card shows the message box's focus once; the box itself has none. -->
  <div
    class="rounded-xl border bg-card shadow-sm transition has-[textarea:focus]:border-ring has-[textarea:focus]:ring-3 has-[textarea:focus]:ring-ring/20"
  >
    {#if menuOpen}
      <div
        id="chat-commands"
        class="max-h-52 overflow-y-auto border-b p-1"
        role="listbox"
        aria-label="命令"
      >
        {#each matches as command, i (command.name)}
          <ListButton
            variant="suggestion"
            role="option"
            aria-selected={i === active}
            class="flex w-full min-w-0 flex-wrap items-baseline"
            onmousedown={(event) => event.preventDefault()}
            onclick={() => pick(command)}
          >
            <span class="shrink-0 font-mono text-sm">/{command.name}</span>
            {#if command.hint}<span
                class="min-w-0 break-words text-muted-foreground"
                >&lt;{command.hint}&gt;</span
              >{/if}
            <span class="ml-auto min-w-0 truncate text-muted-foreground"
              >{command.description}</span
            >
          </ListButton>
        {/each}
      </div>
    {/if}
    {#if uploading}
      <div
        class="flex items-center gap-2 border-b px-3 py-1.5 text-xs text-muted-foreground"
        role="status"
      >
        <span class="min-w-0 flex-1 truncate"
          >上传 {uploading.name} · {uploading.index}/{uploading.total} ·
          {uploading.percent}%</span
        >
        <Button
          variant="quiet"
          size="icon-sm"
          aria-label="取消上传"
          title="取消上传"
          onclick={() => uploadController?.abort()}
          ><IconX aria-hidden="true" /></Button
        >
      </div>
    {/if}
    {#if attachments.length}
      <ul
        class="flex flex-wrap gap-1 border-b px-2 py-1.5"
        aria-label="待发送附件"
      >
        {#each attachments as item (item.path)}
          <li
            class="flex max-w-full min-w-0 items-center gap-1 rounded-md border bg-muted/50 px-2 py-0.5 text-xs"
          >
            {#if item.kind === "image"}<IconPhoto
                class="size-3.5 shrink-0"
                aria-hidden="true"
              />{:else}<IconFile
                class="size-3.5 shrink-0"
                aria-hidden="true"
              />{/if}
            <span class="min-w-0 truncate">{item.name ?? item.path}</span>
            <Button
              variant="quiet"
              size="icon-xs"
              aria-label={`移除附件 ${item.name ?? item.path}`}
              title="移除附件"
              onclick={() => removeAttachment(item.path)}
              ><IconX aria-hidden="true" /></Button
            >
          </li>
        {/each}
      </ul>
    {/if}
    {#if imageDegraded}
      <p class="border-b px-3 py-1.5 text-xs text-warning" role="status">
        该 agent 不支持图片，图片将按服务器路径发送。
      </p>
    {/if}
    <div class="px-2 pt-1">
      <Textarea
        bind:ref={textarea}
        variant="bare"
        rows={2}
        class="max-h-48 min-h-14 resize-none"
        placeholder={unavailable ||
          (locked
            ? "草稿正在提交到终端，确认结果前不能编辑"
            : queueing
              ? "继续输入，发送后加入排队"
              : "给 Agent 发消息，输入 / 查看命令")}
        aria-label="消息"
        aria-controls={menuOpen ? "chat-commands" : undefined}
        aria-expanded={menuOpen}
        {disabled}
        value={draft.text}
        oninput={(event) => {
          const input = event.currentTarget;
          drafts.edit(draft, input.value, input.selectionStart);
          highlighted = 0;
        }}
        onkeydown={keydown}
        onpaste={paste}
      />
    </div>
    <Input
      bind:ref={fileInput}
      class="hidden"
      type="file"
      multiple
      aria-hidden="true"
      onchange={(event) => {
        pickFiles(event.currentTarget.files);
        event.currentTarget.value = "";
      }}
    />
    <!-- Only what acts on the message stays in the card: context use and the
       send controls (they wrap only at very large text). -->
    <div class="flex flex-wrap items-center justify-end gap-1 px-2 pb-2">
      {#if chat.usage}<UsageRing usage={chat.usage} />{/if}
      <Button
        variant="quiet"
        size="icon-touch"
        aria-label="添加附件"
        title="添加附件"
        disabled={disabled || !!uploading}
        onclick={() => fileInput?.click()}
        ><IconPaperclip aria-hidden="true" /></Button
      >
      {#if steerable}
        <Button
          variant="outline"
          size="icon-touch"
          aria-label="插话"
          title="插话：注入当前回合（Ctrl+Enter）"
          disabled={disabled || !ready || sending || !!uploading}
          onclick={() => submit("steer")}
          ><IconMessageBolt aria-hidden="true" /></Button
        >
      {/if}
      {#if running}
        <Button
          variant="secondary"
          size="icon-touch"
          aria-label="停止"
          title="停止当前回合"
          disabled={!!unavailable}
          onclick={stop}><IconPlayerStopFilled aria-hidden="true" /></Button
        >
      {/if}
      <Button
        size="icon-touch"
        aria-label={queueing ? "加入排队" : "发送"}
        title={queueing ? "加入排队" : "发送"}
        disabled={disabled || !ready || sending || !!uploading}
        onclick={() => submit("auto")}
        ><IconArrowUp aria-hidden="true" /></Button
      >
    </div>
  </div>
  <!-- Session options sit under the card; on a narrow screen they wrap
       there instead of crowding the message box. -->
  <div
    class="flex min-w-0 flex-wrap items-center gap-x-0.5 px-1"
    role="toolbar"
    aria-label="会话选项"
  >
    <ChatPermissionMode
      mode={chat.permissionMode}
      disabled={!!unavailable || savingMode}
      onchange={(mode) => void setPermissionMode(mode)}
    />
    <ChatConfig
      config={chat.config}
      {disabled}
      onchange={(configId, value) => {
        actions.setConfig(configId, value).then(
          () => onerror(""),
          (err: unknown) => onerror(errorMessage(err)),
        );
      }}
    />
  </div>
</div>
