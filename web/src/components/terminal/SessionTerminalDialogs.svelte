<script lang="ts">
  import type { HybridSession } from "../../hooks/sessions/hybrid.svelte";
  import type {
    DraftOperation,
    TerminalDraft,
    TerminalDrafts,
    TextInsertion,
  } from "../../domains/terminal/terminal-draft.svelte";
  import CloseSessionDialog from "../sessions/CloseSessionDialog.svelte";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import type { TerminalFeatures } from "../../features/terminal";
  import ScratchPanel from "./ScratchPanel.svelte";
  import { m } from "$lib/paraglide/messages.js";
  import TerminalCopyDialog from "./TerminalCopyDialog.svelte";
  import TerminalFontDialog from "./TerminalFontDialog.svelte";
  import TerminalPasteDialog from "./TerminalPasteDialog.svelte";

  let {
    id,
    features,
    draft,
    drafts,
    hybrid,
    fontSize,
    setFontSize,
    filePaste,
    pasteOperation,
    insertText,
    focusTerminal,
    scratchSaved,
    onremoved,
    onarchived,
    pasteOpen = $bindable(),
    pasteText = $bindable(),
    copyOpen = $bindable(),
    copyText,
    settingsOpen = $bindable(),
    closing = $bindable(),
    choiceClosed = $bindable(),
    scratchOpen = $bindable(),
  }: {
    id: string;
    features: TerminalFeatures;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    hybrid: HybridSession;
    fontSize: number;
    setFontSize: (size: number) => void;
    filePaste: (event: ClipboardEvent) => boolean;
    pasteOperation: DraftOperation | undefined;
    insertText: (text: string, operation?: DraftOperation) => TextInsertion;
    focusTerminal: (event: Event) => void;
    scratchSaved: string;
    onremoved: () => void;
    onarchived: () => void;
    pasteOpen: boolean;
    pasteText: string;
    copyOpen: boolean;
    copyText: string;
    settingsOpen: boolean;
    closing: boolean;
    choiceClosed: boolean;
    scratchOpen: boolean;
  } = $props();

  // The agent session picked when a switch to chat found several.
  let ambiguousPick = $derived(
    hybrid.ambiguous?.candidates[0]?.agentSessionId ?? "",
  );
</script>

<TerminalPasteDialog
  bind:open={pasteOpen}
  bind:text={pasteText}
  {filePaste}
  send={(text) => insertText(text, pasteOperation)}
  onCloseAutoFocus={focusTerminal}
/>
<TerminalCopyDialog
  bind:open={copyOpen}
  text={copyText}
  onCloseAutoFocus={focusTerminal}
/>
<TerminalFontDialog
  bind:open={settingsOpen}
  {fontSize}
  {setFontSize}
  onCloseAutoFocus={focusTerminal}
/>
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
      () => hybrid.awaitingChoice && !choiceClosed,
      (open) => (choiceClosed = !open)
    }
    note={hybrid.chat.state?.forceSwitchNote ?? ""}
    busy={hybrid.answering}
    onchoose={(choice) => void hybrid.choose(choice)}
  />
{/if}
<ScratchPanel bind:open={scratchOpen} {draft} {drafts} saved={scratchSaved} />
