<script lang="ts">
  import type { Connection } from "../../src/platform/ws";
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
  };
  const uploads = { dragging: false, progress: null };
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
    {uploads}
    {quickMenu}
    {draftToggle}
    {send}
    {toggle}
    lock={() => {}}
    paste={async () => {}}
    keyboard={() => {}}
  />
</div>
