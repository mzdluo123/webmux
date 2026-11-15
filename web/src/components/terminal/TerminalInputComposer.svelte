<script lang="ts">
  import IconInbox from "~icons/tabler/inbox";
  import IconTrash from "~icons/tabler/trash";
  import { tick, type Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import { Textarea } from "$lib/components/ui/textarea";
  import { locale } from "$lib/locale";
  import * as m from "$lib/paraglide/messages.js";
  import { useTextareaDraft } from "../../hooks/terminal/textarea-draft.svelte";
  import { editDraftKey } from "../../domains/terminal/draft-key";
  import type {
    TerminalDraft,
    TerminalDrafts,
  } from "../../domains/terminal/terminal-draft.svelte";

  let {
    draft,
    drafts,
    unavailable,
    target,
    openScratch,
    send,
    resend,
    release,
    filePaste,
    controlNotice,
    keyboardDismissed = () => false,
    voice,
  }: {
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    unavailable: string;
    // The session the draft will be sent to.
    target: string;
    openScratch: () => void;
    send: () => void;
    // Repeats an unconfirmed submission under its original ID.
    resend: () => void;
    // Stops tracking a submission and makes its text editable again.
    release: () => void;
    filePaste: (event: ClipboardEvent) => boolean;
    controlNotice: () => void;
    // True after the IME was open and then dismissed, e.g. by Android Back,
    // while an editable may still keep focus.
    keyboardDismissed?: () => boolean;
    // The microphone, beside the draft's other actions.
    voice?: Snippet;
  } = $props();
  function submissionText(state: string): string {
    if (state === "saved") return m.s4_draft_sub_saved();
    if (state === "sending") return m.s4_draft_sub_sending();
    if (state === "received") return m.s4_draft_sub_received();
    if (state === "unknown") return m.s4_draft_sub_unknown();
    if (state === "partial") return m.s4_draft_sub_partial();
    return "";
  }
  const locked = $derived(!!draft.submission);
  let textarea = $state<HTMLTextAreaElement | null>(null);
  let clearOpen = $state(false);
  let keyboardReopenAttempted = false;
  const selection = useTextareaDraft(
    () => textarea,
    () => draft,
    () => drafts,
  );
  export function focus() {
    textarea?.focus({ preventScroll: true });
  }
  // Toggles the soft keyboard; `beforeOpen` runs just before focusing.
  export function keyboard(beforeOpen?: (target: HTMLElement) => void) {
    const focused = document.activeElement === textarea;
    if (focused && keyboardDismissed() && !keyboardReopenAttempted) {
      // Android can hide the IME with Back while the textarea keeps focus;
      // restart focus once so the keyboard can open again. If the browser
      // still declines, the next press toggles normally.
      keyboardReopenAttempted = true;
      textarea?.blur();
      if (textarea) beforeOpen?.(textarea);
      focus();
      return;
    }
    keyboardReopenAttempted = false;
    if (focused) {
      textarea?.blur();
      return;
    }
    if (textarea) beforeOpen?.(textarea);
    focus();
  }
  export function editKey(input: string) {
    if (selection.composing || locked) return;
    if (!editDraftKey(draft, drafts, input)) controlNotice();
    void tick().then(focus);
  }
  export function terminalKey(event: KeyboardEvent) {
    focus();
    if (event.type !== "keydown" || event.isComposing) return;
    if (event.ctrlKey || event.metaKey || event.altKey) {
      controlNotice();
      return;
    }
    const keys: Record<string, string> = {
      Enter: "\n",
      Backspace: "\x7f",
      Delete: "\x1b[3~",
      Tab: "\t",
      ArrowLeft: "\x1b[D",
      ArrowRight: "\x1b[C",
      ArrowUp: "\x1b[A",
      ArrowDown: "\x1b[B",
      Home: "\x1b[H",
      End: "\x1b[F",
    };
    const key = keys[event.key];
    if (key) editKey(key);
    else if (Array.from(event.key).length === 1) editKey(event.key);
    else if (event.key === "Escape") controlNotice();
  }
</script>

<section class="terminal-composer" aria-label={m.s4_composer_aria()}>
  {#if draft.restoredAt}
    <p class="shrink-0 truncate text-xs text-muted-foreground" role="status">
      {m.s4_draft_restored_banner({
        time: new Date(draft.restoredAt).toLocaleString(locale.current),
        target,
      })}
    </p>
  {/if}
  <Textarea
    bind:ref={textarea}
    value={draft.text}
    readonly={locked}
    aria-label={m.s4_draft_content_aria()}
    code
    class="min-h-0 flex-1 resize-none overflow-y-auto field-sizing-fixed"
    oninput={(event) =>
      drafts.edit(
        draft,
        event.currentTarget.value,
        event.currentTarget.selectionStart,
        event.currentTarget.selectionEnd,
      )}
    {...selection.selectionProps}
    onpaste={(event) => {
      filePaste(event);
    }}
    onkeydown={(event) => {
      if (event.isComposing) return;
      if ((event.ctrlKey || event.metaKey) && event.key === "Enter")
        event.preventDefault();
    }}
  />
  <div class="flex shrink-0 flex-wrap items-center gap-2">
    {#if draft.submission || unavailable}<span
        class="min-w-0 flex-[1_1_100%] wrap-anywhere sm:flex-[1_1_12rem] text-xs text-muted-foreground"
        role="status"
        aria-label={m.s4_draft_submission_status_aria()}
        >{draft.submission
          ? submissionText(draft.submission.state)
          : unavailable}</span
      >{/if}
    <div
      class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2"
    >
      {#if draft.submission?.state === "unknown"}
        <Button
          variant="outline"
          size="sm"
          disabled={!!unavailable}
          title={m.s4_resend_original_title()}
          onclick={resend}>{m.s4_resend_original()}</Button
        >
      {/if}
      {#if draft.submission?.state === "unknown" || draft.submission?.state === "partial"}
        <Button
          variant="outline"
          size="sm"
          title={m.s4_release_to_draft_title()}
          onclick={release}>{m.s4_release_to_draft()}</Button
        >
      {/if}
      {#if voice}{@render voice()}{/if}
      <Button
        variant="ghost"
        size="sm"
        title={m.s4_open_scratch()}
        disabled={selection.composing}
        aria-label={draft.scratch.length
          ? m.s4_open_scratch_with_count({
              count: String(draft.scratch.length),
            })
          : m.s4_open_scratch()}
        onclick={openScratch}
        ><IconInbox aria-hidden="true" /><span class="sr-only"
          >{m.s4_scratch_title()}</span
        >{#if draft.scratch.length}<span
            class="rounded-sm bg-muted px-1.5 text-xs tabular-nums"
            aria-hidden="true">{draft.scratch.length}</span
          >{/if}</Button
      >
      <Button
        variant="ghost"
        size="icon-sm"
        title={m.s4_clear_draft()}
        aria-label={m.s4_clear_draft()}
        disabled={!draft.text || selection.composing}
        onclick={() => (clearOpen = true)}
        ><IconTrash aria-hidden="true" /></Button
      >
      {#if !draft.submission}
        <Button
          size="sm"
          disabled={!draft.text || !!unavailable || selection.composing}
          onclick={() => {
            if (!selection.composing) send();
          }}>{m.s4_send()}</Button
        >
      {/if}
    </div>
  </div>
</section>

<ConfirmDialog
  bind:open={clearOpen}
  title={m.s4_clear_draft_title()}
  confirmLabel={m.s4_confirm_clear()}
  destructive
  onconfirm={() => drafts.clear(draft)}>{m.s4_clear_draft_desc()}</ConfirmDialog
>
