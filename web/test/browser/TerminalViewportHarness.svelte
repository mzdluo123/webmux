<script lang="ts">
  import type { Connection } from "../../src/platform/ws";
  import type { UploadProgress } from "../../src/hooks/shared/upload.svelte";
  import FileUploadProgress from "../../src/components/shared/FileUploadProgress.svelte";
  import TerminalFindBar from "../../src/components/terminal/TerminalFindBar.svelte";
  import TerminalViewport from "../../src/components/terminal/TerminalViewport.svelte";

  let {
    connection = "reconnecting" as Connection,
    restoring = false,
    atBottom = true,
    visible = true,
    width = 400,
    height = 240,
    selection = "",
    controlling = true,
    touchGamepad = false,
    send = () => {},
    toggle = () => {},
    setTouchGamepad = () => {},
    scrollToBottom = () => {},
    findOpen = false,
    uploadProgress = null,
    cancelUpload = () => {},
  }: {
    connection?: Connection;
    restoring?: boolean;
    atBottom?: boolean;
    visible?: boolean;
    width?: number;
    height?: number;
    selection?: string;
    controlling?: boolean;
    touchGamepad?: boolean;
    send?: (input: string) => void;
    toggle?: (key: "ctrl" | "alt" | "shift") => void;
    setTouchGamepad?: (value: boolean) => void;
    scrollToBottom?: () => void;
    findOpen?: boolean;
    uploadProgress?: UploadProgress | null;
    cancelUpload?: () => void;
  } = $props();

  const terminal = {
    get selection() {
      return selection;
    },
    get restoring() {
      return restoring;
    },
    get atBottom() {
      return atBottom;
    },
    get connection() {
      return connection;
    },
    get controlling() {
      return controlling;
    },
    exitCode: null,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    copySelection() {},
    clearSelection() {},
    scrollToBottom() {
      scrollToBottom();
    },
    canFind: true,
    findResults: undefined,
    findReplayed: false,
    bufferType: "normal" as const,
    scrollback: 1000,
    historyTruncated: false,
    findNext() {
      return false;
    },
    findPrevious() {
      return false;
    },
    clearFind() {},
  };
  const uploads = {
    dragging: false,
    get progress() {
      return uploadProgress;
    },
  };
</script>

{#snippet quickMenu()}<span></span>{/snippet}
{#snippet draftToggle()}<span></span>{/snippet}

<div
  class="session-terminal force-soft-keys"
  style={`position: relative; width: ${width}px; height: ${height}px`}
>
  <TerminalViewport
    {terminal}
    {visible}
    {touchGamepad}
    {setTouchGamepad}
    {findOpen}
    {uploads}
    {quickMenu}
    {draftToggle}
    {send}
    {toggle}
    lock={() => {}}
    paste={async () => {}}
    keyboard={() => {}}
  >
    {#snippet findBar()}
      <TerminalFindBar open={findOpen} {terminal} onclose={() => {}} />
    {/snippet}
    {#snippet uploadProgress()}
      {#if uploads.progress}
        <FileUploadProgress
          variant="floating"
          offset={findOpen}
          progress={uploads.progress}
          cancel={cancelUpload}
        />
      {/if}
    {/snippet}
  </TerminalViewport>
</div>
