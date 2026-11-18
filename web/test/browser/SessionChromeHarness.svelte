<script lang="ts">
  import { untrack } from "svelte";
  import { useSessionTerminalChrome } from "../../src/hooks/terminal/session-terminal-chrome.svelte";
  import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";

  // The session chrome hook on a chat view, with the terminal's own plumbing
  // stubbed out: the page owns the IME dismissal flag and the chat view owns
  // focus release, so the test drives both from the outside.
  let {
    dismissed = false,
    expose,
  }: {
    dismissed?: boolean;
    expose?: (api: {
      setEditing: (value: boolean) => void;
      editing: () => boolean;
      releases: () => number;
    }) => void;
  } = $props();
  // The test wires its handle once at mount; `dismissed` stays reactive.
  const onExpose = untrack(() => expose);

  const drafts = createTerminalDrafts();
  const draft = drafts.get("chrome-fixture");
  let releases = 0;
  const chrome = useSessionTerminalChrome({
    id: () => "chrome-fixture",
    draft,
    drafts,
    isMounted: () => true,
    isActiveAndVisible: () => true,
    isTerminalView: () => false,
    isTerminalVisible: () => false,
    isDesktop: () => true,
    focusFindBar: () => {},
    focusComposer: () => {},
    clearTerminalFind: () => {},
    focusFallback: () => {},
    fitTerminal: () => {},
    viewportHeight: () => 800,
    keyboardVisibility: () => "hidden",
    keyboardDismissed: () => dismissed,
    releaseChatEditing: () => {
      releases += 1;
    },
    isCodePanelPoppedOut: () => false,
    insertText: () => "draft",
    uploadFiles: () => Promise.resolve(),
    setError: () => {},
    oninfo: () => {},
  });
  chrome.syncSessionEffects(
    {
      view: "chat",
      missing: false,
      info: undefined,
      awaitingChoice: false,
      chat: { connection: "connected" },
    },
    () => "connected",
    () => undefined,
  );
  onExpose?.({
    setEditing: (value) => {
      chrome.chatEditing = value;
    },
    editing: () => chrome.chatEditing,
    releases: () => releases,
  });
</script>

<div></div>
