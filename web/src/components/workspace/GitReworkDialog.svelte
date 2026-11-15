<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconMessageForward from "~icons/tabler/message-forward";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as RadioGroup from "$lib/components/ui/radio-group";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
  import { formatLineRangeLabel } from "../../domains/workspace/git-rework-model";
  import type { GitSessionHandoffMode } from "../../hooks/workspace/git-rework-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    mode = "rework",
    items,
    stagedChanges = [],
    activeSessions,
    selectedSessionId = $bindable(""),
    checkingFreshness = false,
    stalePaths = [],
    staleConfirmed = false,
    copiedFeedback = false,
    hasCommitMessageDraft = false,
    onUpdateNote,
    onRemoveItem,
    onClearAll,
    onCopyMarkdown,
    onConfirmStale,
    onDeliverToSession,
    onStartNewSessionWithDraft,
    onClose,
  }: {
    mode?: GitSessionHandoffMode;
    items: readonly GitReworkItem[];
    stagedChanges?: readonly GitWorkingChange[];
    activeSessions: readonly SessionInfo[];
    selectedSessionId?: string;
    checkingFreshness?: boolean;
    stalePaths?: readonly string[];
    staleConfirmed?: boolean;
    copiedFeedback?: boolean;
    hasCommitMessageDraft?: boolean;
    onUpdateNote: (itemId: string, note: string) => void;
    onRemoveItem: (itemId: string) => void;
    onClearAll: () => void;
    onCopyMarkdown: () => void;
    onConfirmStale: () => void;
    onDeliverToSession: (session: SessionInfo) => void;
    onStartNewSessionWithDraft: () => void;
    onClose: () => void;
  } = $props();

  const isCommitAssist = $derived(mode === "commit-assist");
  const dialogTitle = $derived(
    isCommitAssist
      ? m.s6_rework_title_assist()
      : m.s6_rework_title_drafts({ count: String(items.length) }),
  );
  const dialogDescription = $derived(
    isCommitAssist ? m.s6_rework_desc_assist() : m.s6_rework_desc_drafts(),
  );

  const chosenSession = $derived(
    activeSessions.find((s) => s.id === selectedSessionId),
  );
  const canSubmitPayload = $derived(
    isCommitAssist ? stagedChanges.length > 0 : items.length > 0,
  );
  const canSendToSession = $derived(
    Boolean(chosenSession) && canSubmitPayload && !checkingFreshness,
  );

  function sessionTitle(session: SessionInfo): string {
    return session.title.trim() || session.cmd.join(" ") || session.id;
  }

  function sessionViewLabel(session: SessionInfo): string {
    return session.view === "chat"
      ? m.s6_rework_view_chat()
      : m.s6_rework_view_tui();
  }

  function sessionStatusLabel(status: SessionInfo["status"]): string {
    switch (status) {
      case "running":
        return m.s6_rework_status_running();
      case "waiting":
        return m.s6_rework_status_waiting();
      case "idle":
        return m.s6_rework_status_idle();
      case "exited":
        return m.s6_rework_status_exited();
    }
  }
</script>

<ResponsiveDialog
  title={dialogTitle}
  description={dialogDescription}
  drawerDescription={isCommitAssist
    ? m.s6_rework_drawer_desc_assist()
    : m.s6_rework_drawer_desc_drafts()}
  pinDrawerHeader
  truncateDrawerTitle
  busy={checkingFreshness}
  class="sm:max-w-2xl"
  onclose={onClose}
>
  <div class="min-w-0 space-y-4 text-xs" data-testid="git-rework-dialog">
    {#if hasCommitMessageDraft && !isCommitAssist}
      <InlineNotice tone="info">
        {m.s6_rework_commit_draft_preserved()}
      </InlineNotice>
    {/if}

    {#if isCommitAssist}
      <div class="space-y-2 rounded-md border bg-background p-3">
        <div class="flex items-center justify-between gap-2">
          <span class="font-semibold text-foreground">
            {m.s6_rework_staged_summary_heading({
              count: String(stagedChanges.length),
            })}
          </span>
          <Badge variant="info">Git index</Badge>
        </div>
        {#if stagedChanges.length === 0}
          <p class="text-muted-foreground">
            {m.s6_rework_staged_empty()}
          </p>
        {:else}
          <ScrollArea viewportClasses="max-h-40">
            <ul class="space-y-1 font-mono">
              {#each stagedChanges as item (item.id)}
                <li class="truncate text-foreground">
                  [{item.status}] {item.oldPath && item.oldPath !== item.path
                    ? `${item.oldPath} → ${item.path}`
                    : item.path}
                </li>
              {/each}
            </ul>
          </ScrollArea>
        {/if}
      </div>
    {:else if items.length === 0}
      <div class="rounded-md border bg-background p-4">
        <p class="text-muted-foreground">
          {m.s6_rework_empty_drafts()}
        </p>
      </div>
    {:else}
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <span class="font-semibold text-foreground">
            {m.s6_rework_items_heading({ count: String(items.length) })}
          </span>
          <Button size="sm" variant="ghost" onclick={onClearAll}>
            <IconTrash aria-hidden="true" />
            {m.s6_rework_clear_all()}
          </Button>
        </div>

        <ScrollArea viewportClasses="max-h-72">
          <div class="space-y-2.5 pr-0.5">
            {#each items as item, idx (item.id)}
              <div
                class="space-y-2 rounded-md border bg-background p-2.5"
                data-testid="git-rework-item"
                data-rework-path={item.path}
              >
                <div
                  class="flex min-w-0 flex-wrap items-center justify-between gap-1.5"
                >
                  <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                    <Badge variant="outline">#{idx + 1}</Badge>
                    <span
                      class="min-w-0 truncate font-mono font-medium text-foreground"
                      title={item.path}
                    >
                      {item.path}
                    </span>
                    <Badge
                      variant={item.sourceKind === "worktree"
                        ? "info"
                        : "secondary"}
                      class="max-w-full"
                    >
                      <span class="truncate">{item.versionLabel}</span>
                    </Badge>
                    <Badge variant="outline">
                      <span class="font-mono">
                        {formatLineRangeLabel(item.startLine, item.endLine)}
                      </span>
                    </Badge>
                  </div>

                  <Button
                    size="icon-sm"
                    variant="ghost"
                    aria-label={m.s6_rework_remove_item_aria({
                      path: item.path,
                    })}
                    title={m.s6_rework_remove_item_aria({ path: item.path })}
                    onclick={() => onRemoveItem(item.id)}
                  >
                    <IconTrash aria-hidden="true" />
                  </Button>
                </div>

                {#if item.codeSnippet.trim()}
                  <div class="rounded-sm border bg-background">
                    <ScrollArea viewportClasses="max-h-28">
                      <pre
                        class="p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all text-foreground">{item.codeSnippet}</pre>
                    </ScrollArea>
                  </div>
                {/if}

                <Textarea
                  rows={2}
                  value={item.userNote}
                  placeholder={m.s6_rework_note_placeholder()}
                  aria-label={m.s6_rework_note_item_aria({
                    index: String(idx + 1),
                  })}
                  oninput={(e) => onUpdateNote(item.id, e.currentTarget.value)}
                />
              </div>
            {/each}
          </div>
        </ScrollArea>
      </div>
    {/if}

    {#if stalePaths.length > 0 && !staleConfirmed}
      <InlineNotice tone="warning">
        <div
          class="flex flex-wrap items-center justify-between gap-2"
          data-testid="git-rework-stale-warning"
        >
          <span>
            {m.s6_rework_stale_warning({
              paths: stalePaths.join(m.s6_list_comma_sep()),
            })}
          </span>
          <Button
            size="sm"
            variant="outline"
            onclick={onConfirmStale}
            data-testid="git-rework-confirm-stale"
          >
            {m.s6_rework_confirm_stale_btn()}
          </Button>
        </div>
      </InlineNotice>
    {/if}

    <section class="min-w-0 space-y-2 border-t pt-3">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <span class="font-semibold text-foreground">
          {m.s6_rework_target_session_heading()}
        </span>
        {#if activeSessions.length >= 2}
          <Badge variant="warning" class="max-w-full">
            <span class="truncate">{m.s6_rework_must_pick_session()}</span>
          </Badge>
        {/if}
      </div>

      {#if activeSessions.length === 0}
        <InlineNotice tone="info">
          {m.s6_rework_no_active_sessions()}
        </InlineNotice>
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={!canSubmitPayload}
            onclick={onCopyMarkdown}
            aria-label={m.s6_rework_copy_md_full()}
            title={m.s6_rework_copy_md_full()}
            data-testid="git-rework-copy-markdown"
          >
            {#if copiedFeedback}
              <IconCheck aria-hidden="true" />
              <span>{m.s6_copied()}</span>
            {:else}
              <IconCopy aria-hidden="true" />
              <span class="sm:hidden">{m.s6_rework_copy_short()}</span>
              <span class="max-sm:hidden">{m.s6_rework_copy_md_full()}</span>
            {/if}
          </Button>
          <Button
            size="sm"
            variant="default"
            disabled={!canSubmitPayload}
            onclick={onStartNewSessionWithDraft}
            aria-label={m.s6_rework_new_session_full()}
            title={m.s6_rework_new_session_full()}
            data-testid="git-rework-new-session"
          >
            <IconPlus aria-hidden="true" />
            <span class="sm:hidden">{m.s6_rework_new_session_short()}</span>
            <span class="max-sm:hidden">{m.s6_rework_new_session_full()}</span>
          </Button>
        </div>
      {:else if activeSessions.length === 1}
        {@const single = activeSessions[0]!}
        <div
          class="flex min-w-0 flex-wrap items-center justify-between gap-2 rounded-md border border-foreground/40 bg-background px-3 py-2.5"
          data-testid="git-rework-single-session"
        >
          <div class="min-w-0 space-y-0.5">
            <p class="truncate text-sm font-medium text-foreground">
              {sessionTitle(single)}
            </p>
            <p class="text-xs text-muted-foreground">
              {m.s6_rework_single_session_hint()}
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-1.5">
            <Badge variant="info">{sessionViewLabel(single)}</Badge>
            <Badge variant="outline">{sessionStatusLabel(single.status)}</Badge>
          </div>
        </div>
      {:else}
        <p class="text-muted-foreground">
          {m.s6_rework_multi_session_prompt({
            count: String(activeSessions.length),
          })}
        </p>
        <RadioGroup.Root
          bind:value={selectedSessionId}
          aria-label={m.s6_rework_session_picker_aria()}
          data-testid="git-rework-session-picker"
        >
          {#each activeSessions as session (session.id)}
            <RadioGroup.Item
              value={session.id}
              aria-label={sessionTitle(session)}
              title={sessionTitle(session)}
              description={`${sessionViewLabel(session)} · ${sessionStatusLabel(session.status)} · ID ${session.id}`}
            />
          {/each}
        </RadioGroup.Root>
      {/if}
    </section>

    {#if activeSessions.length > 0}
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-t pt-3"
      >
        <Button
          size="default"
          wrap
          variant="outline"
          disabled={!canSubmitPayload}
          onclick={onCopyMarkdown}
          data-testid="git-rework-copy-markdown"
        >
          {#if copiedFeedback}
            <IconCheck aria-hidden="true" />
            {m.s6_copied()}
          {:else}
            <IconCopy aria-hidden="true" />
            {m.s6_rework_copy_markdown_btn()}
          {/if}
        </Button>

        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <Button size="default" wrap variant="outline" onclick={onClose}>
            {m.s6_close()}
          </Button>
          <Button
            size="default"
            wrap
            variant="default"
            disabled={!canSendToSession}
            onclick={() => chosenSession && onDeliverToSession(chosenSession)}
            aria-label={m.s6_rework_send_full()}
            title={m.s6_rework_send_full()}
            data-testid="git-rework-send-session"
          >
            <IconMessageForward aria-hidden="true" />
            {#if checkingFreshness}
              <span>{m.s6_rework_checking_ellipsis()}</span>
            {:else}
              <span class="sm:hidden">{m.s6_rework_send_short()}</span>
              <span class="max-sm:hidden">{m.s6_rework_send_full()}</span>
            {/if}
          </Button>
        </div>
      </div>
    {/if}
  </div>
</ResponsiveDialog>
