<script lang="ts">
  import { onMount } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import "@xterm/xterm/css/xterm.css";
  import { Button } from "../../src/lib/components/ui/button";
  import { Input } from "../../src/lib/components/ui/input";
  import * as Dialog from "../../src/lib/components/ui/dialog";
  import TerminalViewport from "../../src/components/terminal/TerminalViewport.svelte";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";

  let bytes = $state("");
  let backs = $state(0);
  let container = $state<HTMLDivElement>();
  let term: Terminal;
  tvMode.enabled = true;
  useTvNavigation(() => {
    backs++;
    return true;
  });
  onMount(() => {
    term = new Terminal({ cols: 40, rows: 8 });
    term.open(container!);
    const data = term.onData((input) => {
      bytes += input;
    });
    return () => {
      data.dispose();
      term.dispose();
      tvMode.enabled = false;
    };
  });
  const terminal = {
    selection: "",
    restoring: false,
    atBottom: true,
    connection: "connected" as const,
    controlling: true,
    exitCode: null,
    modifiers: { ctrl: false, alt: false, shift: false },
    shiftLocked: false,
    copySelection() {},
    clearSelection() {},
    scrollToBottom() {},
  };
</script>

<div
  class="session-terminal"
  style="position:fixed;inset:0;display:flex;flex-direction:column"
>
  <header data-tv-terminal-toolbar style="display:flex;gap:20px">
    <Button hidden>隐藏布局按钮</Button>
    <Button>导航起点</Button>
    <Input aria-label="草稿" />
    <Dialog.Root>
      <Dialog.Trigger>打开对话框</Dialog.Trigger>
      <Dialog.Content>
        <Dialog.Title>TV 对话框</Dialog.Title>
        <Dialog.Description>返回只关闭当前弹层</Dialog.Description>
        <Input aria-label="对话框文本" />
        <Dialog.Close>取消</Dialog.Close>
      </Dialog.Content>
    </Dialog.Root>
  </header>
  <div style="height:360px">
    <TerminalViewport
      bind:container
      {terminal}
      visible
      uploads={{ dragging: false, progress: null }}
      send={(input) => {
        bytes += input;
      }}
      toggle={() => {}}
      lock={() => {}}
      paste={async () => {}}
      keyboard={() => {}}
      setTouchGamepad={() => {}}
      focusTerminal={() => term.focus()}
      scroll={(lines) => term.scrollLines(lines)}
    >
      {#snippet quickMenu()}<span></span>{/snippet}
      {#snippet draftToggle()}<span></span>{/snippet}
    </TerminalViewport>
  </div>
  <output aria-label="发送字节">{JSON.stringify(bytes)}</output>
  <output aria-label="路由返回">{backs}</output>
</div>
