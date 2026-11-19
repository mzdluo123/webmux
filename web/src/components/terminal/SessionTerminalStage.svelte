<script lang="ts">
  import type { Snippet } from "svelte";
  import type { ButtonSize, ButtonVariant } from "$lib/components/ui/button";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import type { HybridSession } from "../../hooks/sessions/hybrid.svelte";
  import type { TerminalController } from "../../hooks/terminal/terminal.svelte";
  import type {
    ChatUploads,
    useUpload,
  } from "../../hooks/shared/upload.svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { ChatCodeLinkHandler } from "../../domains/code/markdown-link";
  import type { QuickCommand, SessionInfo } from "../../contracts/schemas";
  import type {
    TerminalDraft,
    TerminalDrafts,
  } from "../../domains/terminal/terminal-draft.svelte";
  import DraftToggle from "./DraftToggle.svelte";
  import TerminalFindBar from "./TerminalFindBar.svelte";
  import TerminalHeader from "./TerminalHeader.svelte";
  import TerminalInputComposer from "./TerminalInputComposer.svelte";
  import TerminalNotices from "./TerminalNotices.svelte";
  import TerminalViewport from "./TerminalViewport.svelte";
  import TerminalWorkspace from "./TerminalWorkspace.svelte";
  import type { createHeaderTerminalView } from "../../domains/terminal/session-terminal-io";
  import type { useFullscreen } from "../../hooks/shared/fullscreen.svelte";
  import type { TerminalFeatures } from "../../features/terminal";
  import type ChatViewComponent from "../chat/ChatView.svelte";
  import SessionPanelDock from "./SessionPanelDock.svelte";

  let {
    id,
    features,
    paneActions,
    chatActions,
    headerTerminal,
    fullscreen,
    codeShown = $bindable(),
    panelView = $bindable("code"),
    bottomToolbarShown = $bindable(),
    desktop,
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
  }: {
    id: string;
    features: TerminalFeatures;
    paneActions: Snippet<[() => void]>;
    chatActions?: Snippet;
    headerTerminal: ReturnType<typeof createHeaderTerminalView>;
    fullscreen: ReturnType<typeof useFullscreen>;
    codeShown: boolean;
    panelView: "picker" | "code" | "split";
    bottomToolbarShown: boolean;
    desktop: boolean;
    documentScroll: boolean;
    chatVisible: boolean;
    hybrid: HybridSession;
    terminal: TerminalController;
    uploads: ReturnType<typeof useUpload> | undefined;
    chatUploads: ChatUploads | undefined;
    chatUploadUnavailable: string;
    sendUnavailable: string;
    quickUnavailable: string;
    voiceEnabled: boolean;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    draftSaved: string;
    scratchSaved: string;
    error: string;
    pendingCopy: boolean;
    copyPending: () => void;
    dismissCopy: () => void;
    notice: string;
    showScratch: () => void;
    showFontSize: () => void;
    closeSession: () => void;
    closing: boolean;
    reopen: () => void;
    leave: () => void;
    feedSession?: SessionInfo;
    showForwards: () => void;
    terminalShown: boolean;
    terminalVisible: boolean;
    sizeLocked: boolean;
    setSizeLocked: (value: boolean) => void;
    keyboardSizeHold: boolean;
    setKeyboardSizeHold: (value: boolean) => void;
    keyboardHeld: boolean;
    findOpen: boolean;
    showFind: () => void;
    closeFind: () => void;
    keyboardDismissed: () => boolean;
    toggleDraft: () => void;
    sendDraft: (resend?: boolean) => void;
    releaseDraftSubmission: () => void;
    runQuickCommand: (cmd: QuickCommand) => void;
    manageQuickCommands: () => void;
    focusTerminal: (event: Event) => void;
    insertVoiceResult: (text: string) => void;
    onVoiceError: (message: string) => void;
    softInput: (input: string) => void;
    controlNotice: () => void;
    paste: () => Promise<void>;
    holdBegin: () => void;
    codePane: Snippet;
    review: (path: string) => void;
    openCodeLocation: ChatCodeLinkHandler;
    onChatEditChange: (value: boolean) => void;
    onChooseSwitch: () => void;
    container: HTMLDivElement | undefined;
  } = $props();

  let findPanel = $state<TerminalFindBar>();
  let chatView = $state<ChatViewComponent>();
  let composer = $state<TerminalInputComposer>();
  let panelDock = $state<SessionPanelDock>();
  let header = $state<TerminalHeader>();

  function openPanels() {
    panelDock?.openPanels();
  }

  function closePanel() {
    codeShown = false;
    header?.focusPanels();
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
    size="icon-xs"
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

<TerminalHeader
  bind:this={header}
  {features}
  {chatActions}
  {openPanels}
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
  bind:bottomToolbarShown
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
<TerminalWorkspace
  {codeShown}
  {desktop}
  draftOpen={draft.enabled && hybrid.view === "terminal"}
  onfind={hybrid.view === "terminal" ? showFind : undefined}
>
  {#snippet codePane()}
    <SessionPanelDock
      bind:this={panelDock}
      bind:view={panelView}
      bind:shown={codeShown}
      codeAvailable={!!features.code}
      close={closePanel}
      {paneActions}
    >
      {@render codeContent()}
    </SessionPanelDock>
  {/snippet}
  {#snippet terminalPane()}
    <div
      class={[
        "relative flex flex-col",
        documentScroll ? "min-h-0 flex-1" : "h-full",
      ]}
    >
      <div
        class={hybrid.view === "chat"
          ? "flex shrink-0 flex-col bg-background"
          : "absolute inset-x-0 top-0 z-30 flex flex-col bg-background"}
      >
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
        {#if features.chat && hybrid.connector && hybrid.view === "terminal"}<div
            class="shrink-0 border-b px-4 py-2"
          >
            <features.chat.ConnectorHint
              connector={hybrid.connector}
              detail={m.s4_connector_recheck_hint()}
              ondismiss={() => hybrid.dismissConnector()}
            />
          </div>{/if}
      </div>
      <div
        class="min-h-0 flex-1"
        use:paneEnter={hybrid.known && terminalShown}
        hidden={!terminalShown}
      >
        <TerminalViewport
          bind:container
          {terminal}
          visible={terminalVisible}
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
      {:else if features.chat && hybrid.view === "switching"}
        <features.chat.SwitchProgress
          target={hybrid.target}
          steps={hybrid.steps}
          awaitingChoice={hybrid.awaitingChoice}
          onchoose={onChooseSwitch}
        />
      {/if}
    </div>
  {/snippet}
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
</TerminalWorkspace>
