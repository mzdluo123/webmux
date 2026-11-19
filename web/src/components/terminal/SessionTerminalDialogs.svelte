<script lang="ts">
  import type { HybridSession } from "../../hooks/sessions/hybrid.svelte";
  import type { useSessionTerminalChrome } from "../../hooks/terminal/session-terminal-chrome.svelte";
  import type {
    DraftOperation,
    TerminalDraft,
    TerminalDrafts,
    TextInsertion,
  } from "../../domains/terminal/terminal-draft.svelte";
  import { openTerminalExternalLink } from "../../domains/terminal/terminal-link";
  import CloseSessionDialog from "../sessions/CloseSessionDialog.svelte";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import type { TerminalFeatures } from "../../features/terminal";
  import ScratchPanel from "./ScratchPanel.svelte";
  import { m } from "$lib/paraglide/messages.js";
  import TerminalCopyDialog from "./TerminalCopyDialog.svelte";
  import TerminalPasteDialog from "./TerminalPasteDialog.svelte";

  let {
    id,
    features,
    draft,
    drafts,
    hybrid,
    chrome,
    filePaste,
    insertText,
    focusTerminal,
    scratchSaved,
    onremoved,
    onarchived,
    closing = $bindable(),
    scratchOpen = $bindable(),
  }: {
    id: string;
    features: TerminalFeatures;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    hybrid: HybridSession;
    chrome: ReturnType<typeof useSessionTerminalChrome>;
    filePaste: (event: ClipboardEvent) => boolean;
    insertText: (text: string, operation?: DraftOperation) => TextInsertion;
    focusTerminal: (event: Event) => void;
    scratchSaved: string;
    onremoved: () => void;
    onarchived: () => void;
    closing: boolean;
    scratchOpen: boolean;
  } = $props();

  // The agent session picked when a switch to chat found several.
  let ambiguousPick = $derived(
    hybrid.ambiguous?.candidates[0]?.agentSessionId ?? "",
  );
</script>

<TerminalPasteDialog
  bind:open={() => chrome.pasteOpen, (v) => (chrome.pasteOpen = v)}
  bind:text={() => chrome.pasteText, (v) => (chrome.pasteText = v)}
  {filePaste}
  send={(text) => insertText(text, chrome.pasteOperation)}
  onCloseAutoFocus={focusTerminal}
/>
<TerminalCopyDialog
  bind:open={() => chrome.copyOpen, (v) => (chrome.copyOpen = v)}
  text={chrome.copyText}
  onCloseAutoFocus={focusTerminal}
/>
<ConfirmDialog
  bind:open={() => chrome.linkOpen, (v) => (chrome.linkOpen = v)}
  title={m.s4_open_link_title()}
  confirmLabel={m.s4_open_link_confirm()}
  onCloseAutoFocus={focusTerminal}
  onconfirm={() => {
    openTerminalExternalLink(chrome.linkUrl);
  }}
>
  <span class="block space-y-2">
    <span class="block">{m.s4_open_link_desc()}</span>
    <code
      class="block rounded-md bg-muted px-2.5 py-1.5 text-left font-mono text-xs break-all text-foreground"
      >{chrome.linkUrl}</code
    >
    <span class="block">{m.s4_open_link_warning()}</span>
  </span>
</ConfirmDialog>
{#if closing && hybrid.info}<CloseSessionDialog
    session={hybrid.info}
    onclose={() => {
      closing = false;
    }}
    onremoved={() => {
      // A hybrid session's record keeps its draft until it is removed.
      if (!hybrid.known) drafts.remove(id);
      onremoved();
    }}
    {onarchived}
  />{/if}
{#if features.chat}
  <ConfirmDialog
    bind:open={
      () => !!hybrid.confirming,
      (open) => {
        if (!open) hybrid.cancelConfirm();
      }
    }
    title={hybrid.confirming?.title ?? ""}
    onconfirm={() => hybrid.confirm()}>{hybrid.confirming?.body}</ConfirmDialog
  >
  <ConfirmDialog
    bind:open={
      () => !!hybrid.ambiguous,
      (open) => {
        if (!open) hybrid.cancelPick();
      }
    }
    title={m.s4_pick_chat_session_title()}
    onconfirm={() => hybrid.pick(ambiguousPick)}
    >{hybrid.ambiguous?.message}<features.chat.CandidatePicker
      candidates={hybrid.ambiguous?.candidates ?? []}
      bind:picked={ambiguousPick}
      label={m.s4_pick_session_to_open()}
    /></ConfirmDialog
  >
  <features.chat.SwitchChoice
    bind:open={
      () => hybrid.awaitingChoice && !chrome.choiceClosed,
      (open) => (chrome.choiceClosed = !open)
    }
    note={hybrid.chat.state?.forceSwitchNote ?? ""}
    busy={hybrid.answering}
    onchoose={(choice) => void hybrid.choose(choice)}
  />
{/if}
<ScratchPanel bind:open={scratchOpen} {draft} {drafts} saved={scratchSaved} />
