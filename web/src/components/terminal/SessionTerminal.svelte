<script lang="ts">
  import { onMount, tick, untrack, type Snippet } from "svelte";
  import { navigate } from "../../platform/api";
  import { useTerminal } from "../../hooks/terminal/terminal.svelte";
  import { keyboardSizeHold } from "../../hooks/terminal/keyboard-size-hold-preference.svelte";
  import { touchGamepad } from "../../hooks/terminal/touch-gamepad-preference.svelte";
  import { useSessionTerminalChrome } from "../../hooks/terminal/session-terminal-chrome.svelte";
  import { createSessionTerminalDraft } from "../../hooks/terminal/session-terminal-draft";
  import type { KeyboardVisibility } from "../../hooks/shared/viewport.svelte";
  import { terminalSession } from "../../hooks/sessions/terminal-session";
  import type { TerminalPlugins } from "../../plugins/terminal";
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
  import { allowAutomaticInputFocus } from "../../hooks/shared/input-focus";
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
    paneActions: Snippet<[boolean]>;
    keyboardDismissed: () => boolean;
    keyboardVisibility: () => KeyboardVisibility;
    viewportHeight: () => number;
    oninfo: (info: SessionInfo, connection: string) => void;
    onremoved: () => void;
    onleave?: () => void;
    onarchived: () => void;
    showForwards: () => void;
    feedSession?: SessionInfo;
    documentScroll?: boolean;
    onscrollstate?: (state: ChatScrollState) => void;
  } = $props();
  // The page keys the contribution snapshot, including explicit load retries.
  const plugins = untrack(() => initialPlugins);
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
  let codePanel = $state<CodePanelComponent>();
  let paneWidth = $state(0);
  let codeLoaded = $state(false);
  const desktop = $derived(paneWidth >= 1024);
  const input = createSessionTerminalDraft({
    draft,
    drafts: untrack(() => drafts),
    terminal: () => t,
    isMounted: () => mounted,
    isChatView: () => hybrid.view === "chat",
    editComposerKey: (input) => stage?.editComposerKey(input),
    restoreInputFocus: () => chrome.restoreInputFocus(),
    setError: (message) => (error = message),
    setNotice: (message) => (notice = message),
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
    uploadFiles: async (files, operation) => {
      if (!uploads) throw new Error("文件插件未启用");
      await uploads.uploadFiles(Array.from(files), operation);
    },
    setError: (msg) => (error = msg),
    oninfo: (info, conn) => oninfo(info, conn),
    onscrollstate: (state) => onscrollstate?.(state),
  });
  $effect(() => {
    if (plugins.code && chrome.codeShown) codeLoaded = true;
    if (terminalVisible()) {
      if (window.scrollX || window.scrollY) window.scrollTo(0, 0);
      void tick().then(() => t.fit());
    }
  });
  function insertReference(text: string) {
    if (hybrid.view === "chat")
      return input.insertText(text, input.beginInput());
    const result = input.insertText(text);
    if (result === "terminal" && !desktop) {
      chrome.codeShown = false;
      void tick().then(() => t.fit());
    }
    return result;
  }
  export function focus() {
    if (!active || !visible) return;
    if (hybrid.view === "chat") stage?.focusChat();
    else if (draft.enabled) stage?.focusComposer();
    else t.focus();
  }
  export function revealChat() {
    if (!desktop) chrome.codeShown = false;
  }
  export function send(input: string) {
    t.send(input);
  }
  async function runQuickCommand(cmd: QuickCommand) {
    if (!plugins.quickCommands) throw new Error("快捷指令插件未启用");
    return plugins.quickCommands.executeSessionQuickCommand(cmd, {
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
        if (active && visible) stage?.focusComposer();
      },
      key: (event) => stage?.handleTerminalKey(event),
      insertText: (text) => {
        input.insertText(text);
      },
    },
    () => keyboardDismissed(),
    () => chrome.sizeLocked,
    () => active && allowAutomaticInputFocus(),
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
      )
    : terminalSession(t);
  chrome.syncSessionEffects(
    hybrid,
    () => t.connection,
    () => t.info?.generation,
  );
  const sessionId = untrack(() => id);
  const files = plugins.files;
  const chatUploads =
    plugins.chat && files
      ? files.useChatUpload(
          (file, progress, signal) =>
            files.uploadFile(sessionId, file, progress, signal),
          (message) => (error = message),
          {
            get: () => draft.attachments,
            set: (next) => drafts.setAttachments(draft, next),
          },
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
    untrack(() => id),
    t,
    (message) => {
      if (mounted) error = message;
      else if (message) onUploadNotice(`会话 ${id}：${message}`);
    },
    {
      begin: () => drafts.begin(draft),
      destination: (operation) => drafts.destination(draft, operation),
      insert: input.insertText,
    },
  );
  const fullscreen = useFullscreen((message) => (error = message));
  const xr = plugins.immersive?.useXrSessionSource({
    id: untrack(() => id),
    terminal: t,
    chatPanel: plugins.chat?.xrChatPanel,
    quickCommands: () =>
      plugins.quickCommands?.availableCommands(!!plugins.files) ?? [],
    codeAvailable: () => !!plugins.code,
    hybrid,
    draft,
    codePanels: () => codePanel?.xrPanels() ?? [],
    focus,
    editDraft: (input) => stage?.editComposerKey(input),
    editChat: (input) => stage?.editChatKey(input),
    sendDraft: input.sendDraft,
    sendChat: () => stage?.sendChatDraft(),
    toggleDraft: input.toggleDraft,
    showCode: () => (chrome.codeShown = true),
    showChat: revealChat,
    showSwitchChoice: () => (chrome.choiceClosed = false),
    runQuick: runQuickCommand,
    error: (message) => (error = message),
    notice: () =>
      [
        error,
        t.error,
        plugins.chat && (stage?.getChatError() || hybrid.chat.error),
      ]
        .filter(Boolean)
        .join(" "),
  });

  onMount(() => {
    mounted = true;
    input.resumeDraftSubmission();
    return () => {
      mounted = false;
    };
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
>
  {#if plugins.chat}
    <plugins.chat.Handoff
      sourceId={hybrid.info?.conversationId ??
        hybrid.recoverable?.conversationId}
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
    {plugins}
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
    {plugins}
    bind:this={stage}
    bind:container
    bind:notice
    bind:codeShown={
      () => chrome.codeShown, (value) => (chrome.codeShown = value)
    }
    {chatActions}
    bind:forceSoftKeys={
      () => forceSoftKeys, (value) => changeSettings({ forceSoftKeys: value })
    }
    {id}
    {paneActions}
    {headerTerminal}
    {fullscreen}
    {xr}
    {desktop}
    {documentScroll}
    chatVisible={active && visible && (desktop || !chrome.codeShown)}
    {hybrid}
    terminal={t}
    {uploads}
    {chatUploads}
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
      {#if plugins.code && codeLoaded}<plugins.code.Panel
          bind:this={codePanel}
          {id}
          gitPlugin={plugins.git}
          visible={visible && chrome.codeShown}
          connection={hybrid.view === "chat"
            ? hybrid.chat.connection
            : t.connection}
          paste={insertReference}
          draftEnabled={draft.enabled || hybrid.view === "chat"}
          reveal={chrome.codeTarget}
          sessionCwd={hybrid.info?.cwd ?? ""}
          reportedCwd={hybrid.info?.reportedCwd ?? null}
        />{/if}
    {/snippet}
  </SessionTerminalStage>
{/snippet}
