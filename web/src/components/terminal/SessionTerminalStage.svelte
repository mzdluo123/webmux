<script lang="ts">
  import IconChat from "~icons/tabler/message";
  import IconTerminal from "~icons/tabler/terminal";
  import type { ButtonSize, ButtonVariant } from "$lib/components/ui/button";
  import DockPanel from "../shared/DockPanel.svelte";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import { bottomToolbar } from "../../hooks/terminal/bottom-toolbar-preference.svelte";
  import { m } from "$lib/paraglide/messages.js";
  import DraftToggle from "./DraftToggle.svelte";
  import TerminalFindBar from "./TerminalFindBar.svelte";
  import TerminalPaneStatus from "./TerminalPaneStatus.svelte";
  import TerminalHeader from "./TerminalHeader.svelte";
  import TerminalInputComposer from "./TerminalInputComposer.svelte";
  import TerminalNotices from "./TerminalNotices.svelte";
  import TerminalViewport from "./TerminalViewport.svelte";
  import TerminalWorkspace from "./TerminalWorkspace.svelte";
  import type ChatViewComponent from "../chat/ChatView.svelte";
  import type { SessionTerminalStageProps } from "./session-terminal-stage-props";

  let {
    id,
    features,
    panels,
    dockSessionName,
    dockSessionNumber,
    active,
    onactivate,
    chatActions,
    headerTerminal,
    codeShown,
    documentScroll,
    chatVisible,
    hybrid,
    terminal,
    uploads,
    chatUploads,
    chatUploadUnavailable,
    sendUnavailable,
    quickUnavailable,
    voiceEnabled,
    draft,
    drafts,
    draftSaved,
    scratchSaved,
    error,
    pendingCopy,
    copyPending,
    dismissCopy,
    notice = $bindable(),
    showScratch,
    closeSession,
    closing,
    reopen,
    leave,
    feedSession,
    showForwards,
    terminalShown,
    terminalVisible,
    sizeLocked,
    setSizeLocked,
    keyboardHeld,
    findOpen,
    showFind,
    closeFind,
    keyboardDismissed,
    toggleDraft,
    sendDraft,
    releaseDraftSubmission,
    runQuickCommand,
    manageQuickCommands,
    focusTerminal,
    insertVoiceResult,
    onVoiceError,
    softInput,
    controlNotice,
    paste,
    holdBegin,
    codePane: codeContent,
    review,
    openCodeLocation,
    onChatEditChange,
    onChooseSwitch,
    container = $bindable(),
  }: SessionTerminalStageProps = $props();

  let fileInput = $state<{ open(): void }>();
  let findPanel = $state<TerminalFindBar>();
  let chatView = $state<ChatViewComponent>();
  let composer = $state<TerminalInputComposer>();
  const paneName = $derived(
    dockSessionName || headerTerminal.info?.title || id,
  );
  const paneTitle = $derived(
    dockSessionNumber ? `${paneName} · ${dockSessionNumber}` : paneName,
  );

  export function menuRoot() {
    return (
      document.querySelector<HTMLElement>(
        `[data-dock-session-commands="${CSS.escape(id)}"]`,
      ) ??
      document.querySelector<HTMLElement>(
        `[data-dock-panel="${CSS.escape(id)}"]`,
      ) ??
      undefined
    );
  }
  export function focusFind() {
    findPanel?.focus();
  }
  export function focusChat() {
    chatView?.focus();
  }
  export function focusComposer() {
    composer?.focus();
  }
  export function editComposerKey(input: string) {
    composer?.editKey(input);
  }
  export function handleTerminalKey(event: KeyboardEvent) {
    return composer?.terminalKey(event);
  }
  export function openComposerKeyboard(
    beginHold: (target: EventTarget) => void,
  ) {
    composer?.keyboard(beginHold);
  }
  export function releaseChatEditing() {
    chatView?.releaseEditing();
  }
  export function setChatVoiceError(message: string): boolean {
    if (!chatView) return false;
    chatView.setVoiceError(message);
    return true;
  }

  const noticesInTerminal = $derived(terminalVisible && !hybrid.recoverable);
  // Recovery controls may remount while a failed agent launch is reconciled.
  // Only a new visit to the session should initiate another automatic attempt.
  let attemptedResume: string | undefined;
  function claimAutoResume() {
    if (attemptedResume === id) return false;
    attemptedResume = id;
    return true;
  }
</script>

{#snippet voiceButton(
  side: "top" | "bottom",
  size?: ButtonSize,
  variant?: ButtonVariant,
)}
  {#if features.voice}<features.voice.Button
      {size}
      {variant}
      disabled={features.voice.isVoiceDisabled(hybrid, terminal, draft)}
      onmousedown={(event) => {
        if (side === "top" && hybrid.view !== "chat") event.preventDefault();
      }}
      onresult={insertVoiceResult}
      onerror={onVoiceError}
    />{/if}
{/snippet}
{#snippet draftVoice()}
  {@render voiceButton("top", "icon-sm", "ghost")}
{/snippet}
{#snippet quickMenu(
  side: "top" | "bottom",
  size?: ButtonSize,
  variant?: ButtonVariant,
)}
  <!-- In the terminal view the microphone lives in the draft box only. -->
  {#if voiceEnabled && hybrid.view === "chat"}
    {@render voiceButton(side, size, variant)}
  {/if}
  {#if features.quickCommands}<features.quickCommands.Menu
      commands={features.quickCommands.availableCommands(!!features.files)}
      target={hybrid.view === "chat"
        ? "chat"
        : draft.enabled
          ? "draft"
          : "terminal"}
      unavailable={quickUnavailable}
      disabled={hybrid.view === "chat"
        ? hybrid.missing
        : terminal.connection === "missing"}
      {side}
      {size}
      {variant}
      run={runQuickCommand}
      manage={manageQuickCommands}
      onCloseAutoFocus={focusTerminal}
    />{/if}
{/snippet}
{#snippet draftToggle()}
  <DraftToggle
    size="icon-xs"
    class="sm:hidden"
    {draft}
    {draftSaved}
    {scratchSaved}
    disabled={terminal.connection === "missing"}
    toggle={toggleDraft}
  />
{/snippet}
{#snippet recovery()}
  {#if features.chat && hybrid.recoverable}<features.chat.Recovery
      record={hybrid.recoverable}
      {claimAutoResume}
      onresumed={reopen}
      {leave}
    />{/if}
{/snippet}

{#snippet paneHeader()}
  <TerminalHeader
    toolbarMode={bottomToolbar.mode}
    {features}
    {chatActions}
    terminal={headerTerminal}
    {draft}
    uploading={uploads?.progress != null}
    {sizeLocked}
    {setSizeLocked}
    {codeShown}
    {quickMenu}
    openUpload={uploads && features.files ? () => fileInput?.open() : undefined}
    {toggleDraft}
    {showScratch}
    {showFind}
    close={closeSession}
    hybrid={hybrid.known ? hybrid : undefined}
  />
{/snippet}
{#snippet paneNotices()}
  {#if !noticesInTerminal}{@render sessionNotices()}{/if}
  {#if features.chat && hybrid.view === "switching"}
    <features.chat.SwitchProgress
      target={hybrid.target}
      steps={hybrid.steps}
      awaitingChoice={hybrid.awaitingChoice}
      onchoose={onChooseSwitch}
    />
  {/if}
{/snippet}

{#snippet sessionNotices()}
  <div class="flex shrink-0 flex-col bg-background">
    <TerminalNotices
      {leave}
      {terminal}
      {uploads}
      hideUploadProgress={terminalShown}
      {features}
      {draft}
      {draftSaved}
      {error}
      {pendingCopy}
      {copyPending}
      {dismissCopy}
      bind:notice
      {showScratch}
      dismissScratch={() => (draft.scratchUpdatedWhileHidden = false)}
      remove={closeSession}
      notices={hybrid.notices}
      dismiss={(key) => hybrid.dismiss(key)}
      recovery={hybrid.recoverable && !closing ? recovery : undefined}
      session={id}
      listening={hybrid.view === "chat"
        ? (feedSession?.listeningPorts ?? hybrid.info?.listeningPorts)
        : hybrid.view === "terminal"
          ? hybrid.info?.listeningPorts
          : undefined}
      {showForwards}
    />
    {#if features.chat && hybrid.connector && hybrid.view === "terminal"}
      <div class="shrink-0 border-b px-4 py-2">
        <features.chat.ConnectorHint
          connector={hybrid.connector}
          detail={m.s4_connector_recheck_hint()}
          ondismiss={() => hybrid.dismissConnector()}
        />
      </div>
    {/if}
  </div>
{/snippet}

<DockPanel
  id={panels.terminal}
  title={paneTitle}
  icon={hybrid.view === "chat" ? IconChat : IconTerminal}
  {onactivate}
  commands={paneHeader}
>
  {#snippet header()}<TerminalPaneStatus terminal={headerTerminal} />{/snippet}
  {#snippet children(visible)}
    {#if features.files && uploads}<features.files.Input
        bind:this={fileInput}
        upload={uploads.uploadFiles}
      />{/if}
    {@render paneNotices()}
    {#if hybrid.view === "chat"}{@render chatPane()}{/if}
    <div
      class="session-terminal split-terminal relative flex min-h-0 flex-1 flex-col"
      class:hidden={hybrid.view === "chat"}
      data-bottom-toolbar={bottomToolbar.mode}
      data-session-id={id}
      data-visible={visible}
      data-active={active}
      data-input-view={hybrid.view}
    >
      {#if active && noticesInTerminal}
        <div class="absolute inset-x-0 top-0 z-30">
          {@render sessionNotices()}
        </div>
      {/if}
      <TerminalWorkspace
        draftOpen={draft.enabled && terminalShown}
        onfind={terminalShown ? showFind : undefined}
        {terminalPane}
        {draftPane}
      />
    </div>
  {/snippet}
</DockPanel>

{#snippet terminalPane()}
  <div class="h-full min-h-0" use:paneEnter={hybrid.known && terminalShown}>
    <TerminalViewport
      bind:container
      {terminal}
      visible={terminalVisible}
      interactive={terminalShown}
      sizeLocked={sizeLocked || keyboardHeld}
      {findOpen}
      {uploads}
      {quickMenu}
      {draftToggle}
      send={softInput}
      toggle={(key) => {
        if (draft.enabled) controlNotice();
        else terminal.toggleModifier(key);
      }}
      lock={(key) => {
        if (draft.enabled) controlNotice();
        else terminal.lockModifier(key);
      }}
      {paste}
      keyboard={holdBegin}
    >
      {#snippet findBar()}
        <TerminalFindBar
          bind:this={findPanel}
          open={findOpen}
          {terminal}
          onclose={closeFind}
        />
      {/snippet}
      {#snippet uploadProgress()}
        {#if terminalShown && features.files && uploads?.progress}<features.files.Progress
            variant="floating"
            offset={findOpen}
            progress={uploads.progress}
            cancel={() => uploads?.cancel()}
          />{/if}
      {/snippet}
    </TerminalViewport>
  </div>
{/snippet}

{#snippet chatPane()}
  {#if features.chat && chatUploads && hybrid.view === "chat"}
    <div
      class={[
        "flex min-h-0 flex-1 flex-col",
        !documentScroll && "h-full overflow-hidden",
      ]}
      use:paneEnter={true}
    >
      <features.chat.ChatView
        bind:this={chatView}
        sessionId={id}
        visible={chatVisible}
        model={hybrid.chat.model}
        chat={hybrid.chat.state}
        connection={hybrid.chat.connection}
        {draft}
        {drafts}
        uploads={chatUploads}
        uploadUnavailable={chatUploadUnavailable}
        location={hybrid.info}
        actions={hybrid.actions}
        onreview={features.code ? review : undefined}
        onopenlocation={features.code ? openCodeLocation : undefined}
        onterminal={() => hybrid.request("terminal")}
        {quickMenu}
        {documentScroll}
        oneditchange={onChatEditChange}
      />
    </div>
  {/if}
{/snippet}
{@render codeContent()}
{#snippet draftPane()}
  <TerminalInputComposer
    bind:this={composer}
    {draft}
    {drafts}
    {keyboardDismissed}
    unavailable={sendUnavailable}
    target={terminal.info?.title || id}
    openScratch={showScratch}
    send={() => sendDraft()}
    resend={() => sendDraft(true)}
    release={releaseDraftSubmission}
    filePaste={(event) => uploads?.filePaste(event) ?? false}
    {controlNotice}
    voice={voiceEnabled ? draftVoice : undefined}
  />
{/snippet}
