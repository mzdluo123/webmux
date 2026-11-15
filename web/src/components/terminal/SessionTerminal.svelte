<script lang="ts">
  import { onMount, tick, untrack, type Snippet } from "svelte";
  import { navigate } from "../../platform/api";
  import { useTerminal } from "../../hooks/terminal/terminal.svelte";
  import { keyboardSizeHold } from "../../hooks/terminal/keyboard-size-hold-preference.svelte";
  import { touchGamepad } from "../../hooks/terminal/touch-gamepad-preference.svelte";
  import {
    createSessionUploadHandlers,
    useSessionTerminalChrome,
  } from "../../hooks/terminal/session-terminal-chrome.svelte";
  import type { KeyboardVisibility } from "../../hooks/shared/viewport.svelte";
  import { terminalSession } from "../../hooks/sessions/terminal-session";
  import {
    availableTerminalPlugins,
    type TerminalPlugins,
  } from "../../plugins/terminal";
  import { useFullscreen } from "../../hooks/shared/fullscreen.svelte";
  import { isIOS } from "../../platform/touch-input";
  import type { ChatScrollState } from "../../platform/capabilities";
  import { shellUi } from "../../hooks/shared/shell.svelte";
  import type { QuickCommand, SessionInfo } from "../../contracts/schemas";
  import type CodePanelComponent from "../code/CodePanel.svelte";
  import SessionTerminalDialogs from "./SessionTerminalDialogs.svelte";
  import SessionTerminalStage from "./SessionTerminalStage.svelte";
  import {
    settings,
    changeSettings,
  } from "../../domains/settings/settings.svelte";
  import {
    createHeaderTerminalView,
    deriveChatUploadUnavailable,
    deriveQuickUnavailable,
    deriveSendUnavailable,
  } from "../../domains/terminal/session-terminal-io";
  import type { TerminalDrafts } from "../../domains/terminal/terminal-draft.svelte";
  import { createSessionTerminalInput } from "../../hooks/terminal/session-terminal-input.svelte";
  import { allowAutomaticInputFocus } from "../../hooks/shared/input-focus";
  import { m } from "$lib/paraglide/messages.js";
  import { pluginAvailable } from "../../platform/plugin-host.svelte";

  let {
    id,
    plugins: initialPlugins,
    drafts,
    onUploadNotice,
    reopen,
    visible,
    active,
    paneActions,
    keyboardDismissed,
    keyboardVisibility,
    viewportHeight,
    oninfo,
    onremoved,
    onarchived,
    onleave,
    showForwards,
    feedSession,
    subscribeRemoved,
    documentScroll = false,
    onscrollstate,
  }: {
    id: string;
    plugins: TerminalPlugins;
    drafts: TerminalDrafts;
    onUploadNotice: (message: string) => void;
    reopen: () => void;
    visible: boolean;
    active: boolean;
    paneActions: Snippet<[() => void]>;
    keyboardDismissed: () => boolean;
    keyboardVisibility: () => KeyboardVisibility;
    viewportHeight: () => number;
    oninfo: (info: SessionInfo, connection: string) => void;
    onremoved: () => void;
    onleave?: () => void;
    onarchived: () => void;
    showForwards: () => void;
    feedSession?: SessionInfo;
    subscribeRemoved?: (listener: (id: string) => void) => () => void;
    documentScroll?: boolean;
    onscrollstate?: (state: ChatScrollState) => void;
  } = $props();
  // The page keys the contribution snapshot, including explicit load retries.
  const plugins = untrack(() => initialPlugins);
  const availablePlugins = $derived(availableTerminalPlugins(plugins));
  const chatAvailable = $derived(!!availablePlugins.chat);
  const draft = untrack(() => drafts.get(id));
  const leave = () => (onleave ? onleave() : navigate(shellUi.home));
  const voiceEnabled = $derived(
    !!plugins.voice &&
      pluginAvailable("voice.recognize") &&
      plugins.voice.voicePreference.enabled,
  );
  let stage = $state<SessionTerminalStage>();
  let container = $state<HTMLDivElement>();
  let terminalRoot = $state<HTMLDivElement>();
  let error = $state("");
  let notice = $state("");
  let closing = $state(false);
  let settingsOpen = $state(false);
  let scratchOpen = $state(false);
  const forceSoftKeys = $derived(settings.values.forceSoftKeys ?? false);
  let mounted = false;
  let seenHybrid = false;
  let codePanel = $state<CodePanelComponent>();
  let paneWidth = $state(0);
  let codeLoaded = $state(false);
  const desktop = $derived(paneWidth >= 1024);
  const input = createSessionTerminalInput({
    draft,
    drafts: untrack(() => drafts),
    terminal: () => t,
    mounted: () => mounted,
    active: () => active && visible,
    immersive: () => !!xr?.active,
    chat: () => hybrid.view === "chat",
    desktop: () => desktop,
    root: () => terminalRoot,
    focusChat: () => stage?.focusChat(),
    focusComposer: () => stage?.focusComposer(),
    editComposer: (value) => stage?.editComposerKey(value),
    hideCode: () => (chrome.codeShown = false),
    restoreFocus: () => chrome.restoreInputFocus(),
    error: (message) => (error = message),
    notice: (message) => (notice = message),
  });
  const sessionId = untrack(() => id);
  const uploadHandlers = createSessionUploadHandlers({
    id: sessionId,
    draft,
    drafts: untrack(() => drafts),
    isMounted: () => mounted,
    isFilesAvailable: () => !!availablePlugins.files,
    insertText: input.insertText,
    setError: (message) => (error = message),
    onUploadNotice: untrack(() => onUploadNotice),
  });
  const terminalShown = () =>
    hybrid.view === "terminal" || hybrid.view === "recovery";
  const terminalVisible = () =>
    visible && (desktop || !chrome.codeShown) && terminalShown();
  const chrome = useSessionTerminalChrome({
    id: () => id,
    code: plugins.code,
    draft,
    drafts: untrack(() => drafts),
    isMounted: () => mounted,
    isActiveAndVisible: () => active && visible,
    isTerminalView: () => hybrid.view === "terminal",
    isTerminalVisible: terminalVisible,
    isDesktop: () => desktop,
    focusFindBar: () => stage?.focusFind(),
    focusComposer: () => chrome.restoreInputFocus(),
    clearTerminalFind: () => t.clearFind(),
    focusFallback: focus,
    fitTerminal: () => t.fit(),
    viewportHeight: () => viewportHeight(),
    keyboardVisibility: () => keyboardVisibility(),
    keyboardDismissed: () => keyboardDismissed(),
    releaseChatEditing: () => stage?.releaseChatEditing(),
    isCodePanelPoppedOut: () => !!codePanel?.isPoppedOut(),
    insertText: input.insertText,
    uploadFiles: uploadHandlers.uploadClipboardFiles(() => uploads),
    setError: (msg) => (error = msg),
    oninfo: (info, conn) => oninfo(info, conn),
    onscrollstate: (state) => onscrollstate?.(state),
  });
  $effect(() => {
    if (!availablePlugins.code && chrome.panelView === "code")
      chrome.codeShown = false;
    else if (chrome.codeShown && chrome.panelView === "code") codeLoaded = true;
    if (terminalVisible()) {
      if (window.scrollX || window.scrollY) window.scrollTo(0, 0);
      void tick().then(() => t.fit());
    }
  });

  export function focus() {
    input.focus();
  }
  export function revealChat() {
    if (!desktop) chrome.codeShown = false;
  }
  export function send(input: string) {
    t.send(input);
  }
  async function runQuickCommand(cmd: QuickCommand) {
    if (!availablePlugins.quickCommands)
      throw new Error(m.s4_quick_commands_unavailable());
    return availablePlugins.quickCommands.executeSessionQuickCommand(cmd, {
      id,
      isMounted: () => mounted,
      isChatView: () => hybrid.view === "chat",
      draft,
      drafts,
      beginInput: input.beginInput,
      insertText: input.insertText,
      resetModifiers: () => t.resetModifiers(),
      submitTerminal: (subId, text, enter, onProgress) =>
        t.submit(subId, text, enter, onProgress),
      getError: () => error,
      setError: (msg) => (error = msg),
    });
  }
  const t = useTerminal(
    () => container!,
    untrack(() => id),
    {
      paste: () => void chrome.paste(),
      copySelection: (text, userGesture) =>
        void chrome.copyToClipboard(text, userGesture ?? false),
    },
    terminalVisible,
    {
      draftEnabled: () => draft.enabled,
      focusDraft: () => {
        if (input.active()) stage?.focusComposer();
      },
      key: (event) => stage?.handleTerminalKey(event),
      insertText: (text) => input.insertText(text),
    },
    () => keyboardDismissed(),
    () => chrome.sizeLocked,
    () => active && !xr?.active && allowAutomaticInputFocus(),
    false,
    () => chrome.keyboardHeld,
  );
  const sendUnavailable = $derived(deriveSendUnavailable(t));
  const quickUnavailable = $derived(deriveQuickUnavailable(t));
  const hybrid = plugins.chat
    ? plugins.chat.useHybrid(
        untrack(() => id),
        t,
        untrack(() => drafts),
        draft,
        false,
        untrack(() => subscribeRemoved),
        () => chatAvailable,
      )
    : terminalSession(t);
  chrome.syncSessionEffects(
    hybrid,
    () => t.connection,
    () => t.info?.generation,
  );
  const files = plugins.files;
  const chatUploads =
    plugins.chat && files
      ? files.useChatUpload(
          uploadHandlers.sendChatFile(files.uploadFile),
          (message) => (error = message),
          uploadHandlers.attachments,
        )
      : undefined;
  const chatUploadUnavailable = $derived(deriveChatUploadUnavailable(hybrid));
  const headerTerminal = createHeaderTerminalView(hybrid, t, terminalShown);
  function showScratch() {
    draft.scratchUpdatedWhileHidden = false;
    scratchOpen = true;
  }
  const uploads = files?.useUpload(
    () => container!,
    sessionId,
    t,
    uploadHandlers.notifyUploadError,
    uploadHandlers.draftBridge,
    () => !!availablePlugins.files,
  );
  const fullscreen = useFullscreen((message) => (error = message));
  const xr = plugins.immersive?.useXrSessionSource({
    id: sessionId,
    terminal: t,
    chatPanel: plugins.chat?.xrChatPanel,
    quickCommands: () =>
      availablePlugins.quickCommands?.availableCommands(
        !!availablePlugins.files,
      ) ?? [],
    codeAvailable: () => !!availablePlugins.code,
    hybrid,
    draft,
    codePanels: () => codePanel?.xrPanels() ?? [],
    focus: input.focusXr,
    blur: input.blurXr,
    editDraft: (input) => stage?.editComposerKey(input),
    editChat: (input) => stage?.editChatKey(input),
    sendDraft: input.sendDraft,
    sendChat: () => stage?.sendChatDraft(),
    toggleDraft: input.toggleDraft,
    showCode: chrome.showCode,
    showChat: revealChat,
    showSwitchChoice: () => (chrome.choiceClosed = false),
    runQuick: runQuickCommand,
    error: (message) => (error = message),
    notice: () => {
      const chatErr = plugins.chat
        ? stage?.getChatError() || hybrid.chat.error
        : "";
      return [error, t.error, chatErr].filter(Boolean).join(" ");
    },
  });
  $effect(() => {
    const record = hybrid.recoverable;
    if (hybrid.known) seenHybrid = true;
    if (record && hybrid.view === "recovery")
      untrack(() => oninfo(record, t.connection));
    if (hybrid.missing && (seenHybrid || xr?.active)) untrack(onremoved);
  });

  onMount(() => {
    mounted = true;
    input.resumeDraftSubmission();
    return () => (mounted = false);
  });

  function insertVoiceResult(text: string) {
    plugins.voice?.applySessionVoiceResult(text, {
      isMounted: () => mounted,
      isChatView: () => hybrid.view === "chat",
      draft,
      drafts,
      beginInput: input.beginInput,
      insertText: input.insertText,
      setError: (msg) => (error = msg),
      onInserted: () => chrome.restoreInputFocus(),
    });
  }
  const focusTerminal = (event: Event) => chrome.restoreInputFocus(event);
</script>

<div
  class="session-terminal flex h-full min-h-0 min-w-0 flex-col overflow-hidden"
  class:force-soft-keys={forceSoftKeys}
  class:ios={isIOS()}
  bind:clientWidth={paneWidth}
  data-terminal-compact={!desktop}
  bind:this={terminalRoot}
  onkeydowncapture={input.guardXrInput}
  onbeforeinputcapture={input.guardXrInput}
  onpastecapture={input.guardXrInput}
>
  {#if plugins.chat}
    <plugins.chat.Handoff
      sourceId={availablePlugins.chat
        ? (hybrid.info?.conversationId ?? hybrid.recoverable?.conversationId)
        : undefined}
      root={terminalRoot}
      children={terminalStage}
    />
  {:else}
    {@render terminalStage(undefined)}
  {/if}
</div>

{#if active && visible}
  <SessionTerminalDialogs
    {id}
    plugins={availablePlugins}
    {draft}
    {drafts}
    {hybrid}
    fontSize={t.fontSize}
    setFontSize={t.setFontSize}
    filePaste={(event) => uploads?.filePaste(event) ?? false}
    pasteOperation={chrome.pasteOperation}
    insertText={input.insertText}
    {focusTerminal}
    scratchSaved={input.scratchSaved}
    {onremoved}
    {onarchived}
    bind:pasteOpen={() => chrome.pasteOpen, (v) => (chrome.pasteOpen = v)}
    bind:pasteText={() => chrome.pasteText, (v) => (chrome.pasteText = v)}
    bind:copyOpen={() => chrome.copyOpen, (v) => (chrome.copyOpen = v)}
    copyText={chrome.copyText}
    bind:settingsOpen
    bind:closing
    bind:choiceClosed={
      () => chrome.choiceClosed, (v) => (chrome.choiceClosed = v)
    }
    bind:scratchOpen
  />
{/if}

{#snippet terminalStage(chatActions: Snippet | undefined)}
  <SessionTerminalStage
    plugins={availablePlugins}
    bind:this={stage}
    bind:container
    bind:notice
    bind:codeShown={
      () => chrome.codeShown, (value) => (chrome.codeShown = value)
    }
    bind:panelView={
      () => chrome.panelView, (value) => (chrome.panelView = value)
    }
    {chatActions}
    bind:forceSoftKeys={
      () => forceSoftKeys, (value) => changeSettings({ forceSoftKeys: value })
    }
    {id}
    {paneActions}
    {headerTerminal}
    {fullscreen}
    xr={availablePlugins.immersive ? xr : undefined}
    {desktop}
    {documentScroll}
    chatVisible={((active && visible) || (input.xrFocused && !!xr?.active)) &&
      (desktop || !chrome.codeShown || !!xr?.active)}
    {hybrid}
    terminal={t}
    uploads={availablePlugins.files ? uploads : undefined}
    chatUploads={availablePlugins.chat ? chatUploads : undefined}
    {chatUploadUnavailable}
    {sendUnavailable}
    {quickUnavailable}
    {voiceEnabled}
    {draft}
    {drafts}
    draftSaved={input.draftSaved}
    scratchSaved={input.scratchSaved}
    {error}
    pendingCopy={chrome.pendingCopy !== null}
    copyPending={chrome.copyPending}
    dismissCopy={() => (chrome.pendingCopy = null)}
    {showScratch}
    showFontSize={() => (settingsOpen = true)}
    closeSession={() => (closing = true)}
    {closing}
    {reopen}
    {leave}
    {feedSession}
    {showForwards}
    terminalShown={terminalShown()}
    terminalVisible={terminalVisible()}
    sizeLocked={chrome.sizeLocked}
    setSizeLocked={chrome.setSizeLocked}
    keyboardSizeHold={keyboardSizeHold.enabled}
    setKeyboardSizeHold={(value) => (keyboardSizeHold.enabled = value)}
    keyboardHeld={chrome.keyboardHeld}
    touchGamepad={touchGamepad.enabled}
    setTouchGamepad={(value) => (touchGamepad.enabled = value)}
    findOpen={chrome.findOpen}
    showFind={chrome.showFind}
    closeFind={chrome.closeFind}
    {keyboardDismissed}
    toggleDraft={input.toggleDraft}
    sendDraft={input.sendDraft}
    releaseDraftSubmission={input.releaseDraftSubmission}
    runQuickCommand={(cmd) => void runQuickCommand(cmd)}
    manageQuickCommands={() =>
      navigate(`/settings?session=${id}#quick-commands`)}
    {focusTerminal}
    {insertVoiceResult}
    onVoiceError={(message) => {
      if (!mounted) return;
      if (hybrid.view !== "chat" || !stage?.setChatVoiceError(message))
        error = message;
    }}
    softInput={input.softInput}
    controlNotice={input.controlNotice}
    paste={chrome.paste}
    holdBegin={() => {
      if (draft.enabled) stage?.openComposerKeyboard(chrome.beginHold);
      else t.keyboard(chrome.beginHold);
    }}
    review={chrome.review}
    openCodeLocation={chrome.openCodeLocation}
    onChatEditChange={(value) => (chrome.chatEditing = value)}
    onChooseSwitch={() => (chrome.choiceClosed = false)}
  >
    {#snippet codePane()}
      {#if availablePlugins.code && codeLoaded}<availablePlugins.code.Panel
          bind:this={codePanel}
          {id}
          gitPlugin={availablePlugins.git}
          visible={visible && chrome.codeShown && chrome.panelView === "code"}
          connection={hybrid.view === "chat"
            ? hybrid.chat.connection
            : t.connection}
          paste={input.insertReference}
          draftEnabled={draft.enabled || hybrid.view === "chat"}
          reveal={chrome.codeTarget}
          sessionCwd={hybrid.info?.cwd ?? ""}
          reportedCwd={hybrid.info?.reportedCwd ?? null}
        />{/if}
    {/snippet}
  </SessionTerminalStage>
{/snippet}
