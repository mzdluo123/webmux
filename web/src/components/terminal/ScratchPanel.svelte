<script lang="ts">
  import { tick } from "svelte";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconArrowUpRight from "~icons/tabler/arrow-up-right";
  import IconDots from "~icons/tabler/dots";
  import IconInbox from "~icons/tabler/inbox";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Button } from "$lib/components/ui/button";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import ScratchNoteEditor from "./ScratchNoteEditor.svelte";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Dialog from "$lib/components/ui/dialog";
  import { locale } from "$lib/locale";
  import { m } from "$lib/paraglide/messages.js";
  import {
    scratchEntriesLimit,
    scratchEntryLimit,
    scratchStorageLimit,
  } from "../../domains/terminal/draft-storage";
  import type {
    ScratchAddResult,
    ScratchTakeMode,
    ScratchTakeResult,
    TerminalDraft,
    TerminalDrafts,
  } from "../../domains/terminal/terminal-draft.svelte";

  let {
    open = $bindable(false),
    draft,
    drafts,
    saved,
  }: {
    open: boolean;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    // Where the scratch list is kept, e.g. "已保存在本机".
    saved: string;
  } = $props();
  let closeButton = $state<HTMLButtonElement | null>(null);
  let newText = $state("");
  let newEditor = $state<HTMLTextAreaElement | null>(null);
  let noteList = $state<HTMLDivElement>();
  let opener: HTMLButtonElement | null = null;
  let conflictTrigger: HTMLElement | undefined;
  let focusNoteId: string | undefined;
  let notice = $state("");
  let noticeTone = $state<"danger" | "success">("danger");
  let conflictId = $state<string | null>(null);
  let clearOpen = $state(false);
  const locked = $derived(!!draft.submission);
  const full = $derived(draft.scratch.length >= scratchEntriesLimit);
  const stale = $derived(
    !!draft.scratchGeneration &&
      !!draft.generation &&
      draft.scratchGeneration !== draft.generation,
  );

  function failure(result: ScratchAddResult | ScratchTakeResult) {
    if (result === "full")
      return m.s4_scratch_err_full({ limit: String(scratchEntriesLimit) });
    if (result === "too-long")
      return m.s4_scratch_err_too_long({
        limit: scratchEntryLimit.toLocaleString(locale.current),
      });
    if (result === "over-total")
      return m.s4_scratch_err_over_total({
        limit: scratchStorageLimit.toLocaleString(locale.current),
      });
    if (result === "locked") return m.s4_scratch_err_locked();
    if (result === "empty") return m.s4_scratch_err_empty();
    return m.s4_scratch_err_failed();
  }
  function add() {
    const result = drafts.addScratch(draft, newText);
    if (result === "added") {
      newText = "";
      newEditor?.focus({ preventScroll: true });
      if (!draft.scratchPersisted) {
        notice = m.s4_scratch_added_unpersisted();
        noticeTone = "danger";
      } else notice = "";
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function stash() {
    const result = drafts.stash(draft);
    if (result === "added") {
      if (!draft.scratchPersisted) {
        notice = m.s4_scratch_stashed_unpersisted();
        noticeTone = "danger";
      } else {
        notice = m.s4_scratch_stashed();
        noticeTone = "success";
      }
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function take(id: string, trigger: HTMLElement) {
    const item = draft.scratch.find((candidate) => candidate.id === id);
    if (!item?.text) {
      notice = m.s4_scratch_item_empty();
      noticeTone = "danger";
      return;
    }
    if (draft.submission) {
      notice = failure("locked");
      noticeTone = "danger";
      return;
    }
    if (draft.text) {
      conflictTrigger = trigger;
      conflictId = id;
      return;
    }
    takeNow(id, "replace");
  }
  function focusAvailableNote() {
    const next = Array.from(
      noteList?.querySelectorAll<HTMLButtonElement>(
        "[data-scratch-take]:enabled",
      ) ?? [],
    ).find((button) => button.dataset.scratchTake === focusNoteId);
    (next ?? newEditor)?.focus();
    focusNoteId = undefined;
  }
  function takeNow(id: string, mode: ScratchTakeMode) {
    const index = draft.scratch.findIndex((item) => item.id === id);
    const fromConflict = conflictId !== null;
    const result = drafts.takeScratch(draft, id, mode);
    conflictId = null;
    if (result === "taken") {
      if (!draft.scratchPersisted || !draft.persisted) {
        notice = m.s4_scratch_taken_unpersisted();
        noticeTone = "danger";
        const available = (item: TerminalDraft["scratch"][number]) =>
          !!item.text;
        focusNoteId = (
          draft.scratch.slice(index).find(available) ??
          draft.scratch.slice(0, index).reverse().find(available)
        )?.id;
        if (!fromConflict) void tick().then(focusAvailableNote);
      } else {
        notice = "";
        open = false;
      }
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function edit(id: string, value: string, element: HTMLTextAreaElement) {
    if (drafts.editScratch(draft, id, value)) {
      if (!draft.scratchPersisted) {
        notice = m.s4_scratch_unpersisted();
        noticeTone = "danger";
      } else if (notice) notice = "";
      return;
    }
    element.value =
      draft.scratch.find((candidate) => candidate.id === id)?.text ?? "";
    notice = m.s4_scratch_edit_limits({
      entryLimit: scratchEntryLimit.toLocaleString(locale.current),
      totalLimit: scratchStorageLimit.toLocaleString(locale.current),
    });
    noticeTone = "danger";
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    class="flex h-[min(48rem,calc(100dvh-1rem))] max-h-[calc(100dvh-1rem)] max-w-[calc(100%-1rem)] flex-col gap-0 overflow-hidden p-0 sm:h-[min(48rem,calc(100dvh-4rem))] sm:max-w-2xl sm:[@media(max-height:480px)]:h-[calc(100dvh-1rem)]"
    showCloseButton={false}
    onOpenAutoFocus={(event) => {
      event.preventDefault();
      const active = document.activeElement;
      opener = active instanceof HTMLButtonElement ? active : null;
      closeButton?.focus();
    }}
    onCloseAutoFocus={(event) => {
      if (!opener?.isConnected) return;
      event.preventDefault();
      opener.focus({ preventScroll: true });
    }}
  >
    <div
      class="flex shrink-0 items-start gap-3 border-b px-4 py-4 sm:px-6 sm:py-5 sm:[@media(max-height:480px)]:py-2"
    >
      <span
        class="hidden size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary sm:flex"
        aria-hidden="true"><IconInbox class="size-5" /></span
      >
      <Dialog.Header class="min-w-0 flex-1 gap-1 pr-0 text-left">
        <Dialog.Title>{m.s4_scratch_title()}</Dialog.Title>
        <Dialog.Description>{m.s4_scratch_description()}</Dialog.Description>
      </Dialog.Header>
      <Dialog.Close>
        {#snippet child({ props })}
          <Button
            {...props}
            bind:ref={closeButton}
            variant="ghost"
            size="icon"
            class="size-11 -mr-2 -mt-1"
            title={m.s4_scratch_close()}
            aria-label={m.s4_scratch_close()}
            ><IconX aria-hidden="true" /></Button
          >
        {/snippet}
      </Dialog.Close>
    </div>

    <div
      class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6"
      data-testid="scratch-scroll"
    >
      <div class="grid min-w-0 gap-3 [&>*]:min-w-0">
        {#if stale}
          <InlineNotice tone="warning"
            >{m.s4_scratch_stale_notice()}</InlineNotice
          >
        {/if}
        {#if locked}
          <InlineNotice tone="info">{m.s4_scratch_locked_notice()}</InlineNotice
          >
        {/if}
        {#if draft.text}
          <div class="flex items-center gap-3 rounded-lg bg-muted/60 p-3">
            <div class="min-w-0 flex-1">
              <p class="text-xs font-medium text-muted-foreground">
                {m.s4_scratch_current_draft()}
              </p>
              <p class="mt-1 truncate text-sm" title={draft.text}>
                {draft.text}
              </p>
            </div>
            <Button
              variant="outline"
              size="icon-touch"
              disabled={locked || full}
              title={m.s4_scratch_stash_current()}
              aria-label={m.s4_scratch_stash_current()}
              onclick={stash}
            >
              <IconInbox aria-hidden="true" />
            </Button>
          </div>
        {/if}
        <div class="flex min-h-11 items-center justify-between gap-2">
          <h3 class="flex items-center gap-2 text-sm font-medium">
            {m.s4_scratch_saved_heading()}
            <span
              class="rounded-md bg-muted px-2 py-0.5 text-xs tabular-nums text-muted-foreground"
              >{draft.scratch.length} / {scratchEntriesLimit}</span
            >
          </h3>
          <Button
            variant="quiet"
            size="icon-touch"
            disabled={!draft.scratch.length}
            title={m.s4_scratch_clear()}
            aria-label={m.s4_scratch_clear()}
            onclick={() => (clearOpen = true)}
            ><IconTrash aria-hidden="true" /></Button
          >
        </div>
        {#if draft.scratch.length}
          <div
            bind:this={noteList}
            class="grid min-w-0 gap-3 [&>*]:min-w-0"
            role="list"
            aria-label={m.s4_scratch_list_label()}
          >
            {#each draft.scratch as item, index (item.id)}
              <div
                class="rounded-lg border bg-card p-3 transition-colors focus-within:border-ring/60"
                role="listitem"
              >
                <div class="mb-2 flex flex-wrap items-center gap-2">
                  <span
                    class="text-xs font-medium tabular-nums text-muted-foreground"
                    >{m.s4_scratch_item_badge({
                      num: String(index + 1).padStart(2, "0"),
                    })}</span
                  >
                  <span
                    class="ml-auto text-xs tabular-nums text-muted-foreground"
                    >{m.s4_scratch_char_count({
                      count: item.text.length.toLocaleString(locale.current),
                    })}</span
                  >
                  <DropdownMenu.Root>
                    <DropdownMenu.Trigger>
                      {#snippet child({ props })}
                        <Button
                          {...props}
                          variant="ghost"
                          size="icon-touch"
                          title={m.s4_more_actions()}
                          aria-label={m.s4_scratch_item_more_actions({
                            index: String(index + 1),
                          })}><IconDots aria-hidden="true" /></Button
                        >
                      {/snippet}
                    </DropdownMenu.Trigger>
                    <DropdownMenu.Content align="end">
                      <DropdownMenu.Item
                        class="min-h-11 sm:min-h-9"
                        disabled={index === 0}
                        aria-label={m.s4_scratch_move_up_aria({
                          index: String(index + 1),
                        })}
                        onSelect={() => drafts.moveScratch(draft, item.id, -1)}
                        ><IconArrowUp
                          aria-hidden="true"
                        />{m.s4_move_up()}</DropdownMenu.Item
                      >
                      <DropdownMenu.Item
                        class="min-h-11 sm:min-h-9"
                        disabled={index === draft.scratch.length - 1}
                        aria-label={m.s4_scratch_move_down_aria({
                          index: String(index + 1),
                        })}
                        onSelect={() => drafts.moveScratch(draft, item.id, 1)}
                        ><IconArrowDown
                          aria-hidden="true"
                        />{m.s4_move_down()}</DropdownMenu.Item
                      >
                      <DropdownMenu.Separator />
                      <DropdownMenu.Item
                        class="min-h-11 sm:min-h-9"
                        variant="destructive"
                        aria-label={m.s4_scratch_delete_aria({
                          index: String(index + 1),
                        })}
                        onSelect={() => drafts.removeScratch(draft, item.id)}
                        ><IconTrash
                          aria-hidden="true"
                        />{m.s4_delete()}</DropdownMenu.Item
                      >
                    </DropdownMenu.Content>
                  </DropdownMenu.Root>
                  <Button
                    variant="outline"
                    size="icon-touch"
                    disabled={!item.text || locked}
                    title={m.s4_scratch_use_in_draft()}
                    aria-label={m.s4_scratch_use_aria({
                      index: String(index + 1),
                    })}
                    data-scratch-take={item.id}
                    onclick={(event) => take(item.id, event.currentTarget)}
                    ><IconArrowUpRight
                      class="text-primary"
                      aria-hidden="true"
                    /></Button
                  >
                </div>
                <Textarea
                  value={item.text}
                  aria-label={m.s4_scratch_item_aria({
                    index: String(index + 1),
                  })}
                  rows={3}
                  maxlength={scratchEntryLimit}
                  variant="ghost"
                  code
                  class="max-h-48 min-h-20 resize-y"
                  oninput={(event) =>
                    edit(
                      item.id,
                      event.currentTarget.value,
                      event.currentTarget,
                    )}
                />
                <div class="mt-2 flex items-center justify-between gap-3">
                  <time
                    class="text-xs text-muted-foreground"
                    datetime={new Date(item.updatedAt).toISOString()}
                    title={new Date(item.updatedAt).toLocaleString(
                      locale.current,
                    )}
                  >
                    {m.s4_scratch_updated_at({
                      time: new Date(item.updatedAt).toLocaleTimeString(
                        locale.current,
                        {
                          hour: "2-digit",
                          minute: "2-digit",
                          hour12: false,
                        },
                      ),
                    })}
                  </time>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div
            class="grid justify-items-center gap-2 rounded-lg border border-dashed px-5 py-8 text-center"
          >
            <IconInbox
              class="mb-1 size-7 text-muted-foreground"
              aria-hidden="true"
            />
            <p class="text-sm font-medium">{m.s4_scratch_empty_title()}</p>
            <p class="max-w-64 text-sm text-muted-foreground">
              {m.s4_scratch_empty_desc()}
            </p>
          </div>
        {/if}
      </div>
    </div>
    {#if notice || full}
      <div class="shrink-0 space-y-2 border-t px-4 py-2 sm:px-6">
        {#if notice}
          <InlineNotice tone={noticeTone} size="compact">{notice}</InlineNotice>
        {/if}
        {#if full}
          <p class="text-xs text-warning" role="status">
            {m.s4_scratch_limit_reached({
              limit: String(scratchEntriesLimit),
            })}
          </p>
        {/if}
      </div>
    {/if}

    <ScratchNoteEditor
      bind:editor={newEditor}
      bind:text={newText}
      {saved}
      {full}
      onadd={add}
    />
  </Dialog.Content>
</Dialog.Root>

<AlertDialog.Root
  open={conflictId !== null}
  onOpenChange={(value) => {
    if (!value) conflictId = null;
  }}
>
  <AlertDialog.Content
    onCloseAutoFocus={(event) => {
      event.preventDefault();
      void tick().then(() => {
        if (!open || conflictId !== null) return;
        if (conflictTrigger?.isConnected) {
          conflictTrigger.focus({ preventScroll: true });
        } else {
          focusAvailableNote();
        }
      });
    }}
  >
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s4_scratch_conflict_title()}</AlertDialog.Title>
      <AlertDialog.Description
        >{m.s4_scratch_conflict_desc()}</AlertDialog.Description
      >
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{m.s4_cancel()}</AlertDialog.Cancel>
      <Button
        variant="outline"
        onclick={() => conflictId && takeNow(conflictId, "append")}
        >{m.s4_scratch_append_draft()}</Button
      >
      <Button onclick={() => conflictId && takeNow(conflictId, "replace")}
        >{m.s4_scratch_replace_draft()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<ConfirmDialog
  bind:open={clearOpen}
  title={m.s4_scratch_clear_title()}
  confirmLabel={m.s4_confirm_clear()}
  destructive
  onconfirm={() => drafts.clearScratch(draft)}
  >{m.s4_scratch_clear_desc()}</ConfirmDialog
>
