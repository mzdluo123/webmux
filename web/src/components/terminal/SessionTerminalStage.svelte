<script lang="ts">
  import type { ButtonSize, ButtonVariant } from "$lib/components/ui/button";
  import DockPanel from "../shared/DockPanel.svelte";
  import { mountDockHost } from "../shared/dock-context";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import { m } from "$lib/paraglide/messages.js";
  import DraftToggle from "./DraftToggle.svelte";
  import TerminalFindBar from "./TerminalFindBar.svelte";
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
    paneIndex,
    toolbar,
    active,
    pickerOpen,
    openPanels,
    onactivate,
    chatActions,
    headerTerminal,
    fullscreen,
    codeShown = $bindable(),
    forceSoftKeys = $bindable(),
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
    showFontSize,
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
    keyboardSizeHold,
    setKeyboardSizeHold,
    keyboardHeld,
    touchGamepad,
    setTouchGamepad,
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

  let findPanel = $state<TerminalFindBar>();
  let chatView = $state<ChatViewComponent>();
  let composer = $state<TerminalInputComposer>();
  const title = $derived(
    terminal.info?.title || headerTerminal.info?.title || m.s4_tab_terminal(),
  );
  const terminalTitle = $derived(
    m.dock_terminal_title({ index: paneIndex + 1, title }),
  );
  const chatTitle = $derived(
    m.dock_chat_title({ index: paneIndex + 1, title }),
  );

  const mountToolbar = (element: HTMLElement) =>
    mountDockHost(element, () => toolbar);

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

<div>
  {#if active}
    <div
      use:mountToolbar
      class="session-terminal"
      class:force-soft-keys={forceSoftKeys}
    >
      <TerminalHeader
        {features}
        {chatActions}
        {openPanels}
        {pickerOpen}
        terminal={headerTerminal}
        {fullscreen}
        {draft}
        {draftSaved}
        {scratchSaved}
        uploading={uploads?.progress != null}
        {sizeLocked}
        {setSizeLocked}
        {keyboardSizeHold}
        {setKeyboardSizeHold}
        bind:codeShown
        bind:forceSoftKeys
        {quickMenu}
        upload={uploads?.uploadFiles}
        {toggleDraft}
        {showScratch}
        {showFontSize}
        {showFind}
        close={closeSession}
        {leave}
        hybrid={hybrid.known ? hybrid : undefined}
      />
      {#if !terminalVisible}{@render sessionNotices()}{/if}
      {#if features.chat && hybrid.view === "switching"}
        <features.chat.SwitchProgress
          target={hybrid.target}
          steps={hybrid.steps}
          awaitingChoice={hybrid.awaitingChoice}
          onchoose={onChooseSwitch}
        />
      {/if}
    </div>
  {/if}
</div>

{#snippet sessionNotices()}
  <div class="flex shrink-0 flex-col bg-background">
    <TerminalNotices
      {leave}
      {terminal}
      {uploads}
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

<DockPanel id={panels.terminal} title={terminalTitle} {onactivate}>
  {#snippet children(visible)}
    <div
      class="session-terminal split-terminal relative flex min-h-0 flex-1 flex-col overflow-hidden"
      class:force-soft-keys={forceSoftKeys}
      data-session-id={id}
      data-visible={visible}
      data-active={active}
      data-input-view={hybrid.view}
    >
      {#if active && terminalVisible}
        <div class="absolute inset-x-0 top-0 z-30">
          {@render sessionNotices()}
        </div>
      {/if}
      {#if !terminalShown && hybrid.view === "chat"}
        <p class="shrink-0 border-b px-2 py-1 text-xs text-muted-foreground">
          {m.dock_terminal_inactive()}
        </p>
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
      {touchGamepad}
      {setTouchGamepad}
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
    </TerminalViewport>
  </div>
{/snippet}

{#if hybrid.known}
  <DockPanel id={panels.chat} title={chatTitle} {onactivate}>
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
    {:else if hybrid.view === "terminal"}
      <p class="p-3 text-sm text-muted-foreground">{m.dock_chat_inactive()}</p>
    {/if}
  </DockPanel>
{/if}
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
