<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import CodeChangesList from "../../src/components/CodeChangesList.svelte";
  import ConfirmDialog from "../../src/components/ConfirmDialog.svelte";
  import SoftKeys from "../../src/components/SoftKeys.svelte";
  import { Button } from "$lib/components/ui/button";
  import QuickCommandMenu from "../../src/components/QuickCommandMenu.svelte";
  import TerminalHeader from "../../src/components/TerminalHeader.svelte";
  import TerminalWorkspace from "../../src/components/TerminalWorkspace.svelte";
  import type { SessionInfo } from "../../src/schemas";
  import { session } from "../fixtures";
  import { createTerminalDrafts } from "../../src/terminal-draft.svelte";
  import ChatView from "./ChatView.svelte";
  import SwitchProgress from "./SwitchProgress.svelte";
  import {
    chatState,
    codeChanges,
    terminalLines,
    type Agent,
    type Scenario,
    type Side,
  } from "./fixtures";

  // The existing session page (header, workspace split and code panel) with
  // a hybrid session from issue #181 in it: the chat view takes the pane the
  // terminal normally fills, and the header switches between the two sides.
  let {
    scenario = "conversation",
    agent = "omp",
  }: { scenario?: Scenario; agent?: Agent } = $props();
  let chat = $state(untrack(() => chatState(scenario, agent)));
  let switching = $state<{ to: Side; step: number } | null>(
    untrack(() => (scenario === "switching" ? { to: "tui", step: 1 } : null)),
  );
  let confirm = $state<{ title: string; body: string; run: () => void }>();
  let confirmOpen = $state(false);
  let codeShown = $state(false);
  let idNoticeDismissed = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  onDestroy(() => clearTimeout(timer));
  const desktopMedia = new MediaQuery("(min-width: 1024px)");
  const drafts = createTerminalDrafts();
  const draft = drafts.get("acp-prototype");

  const pending = $derived(
    chat.timeline.some((e) => e.type === "tool" && e.permission),
  );
  const info = $derived<SessionInfo>({
    ...session,
    id: "acpdemo1",
    title: chat.title,
    cwd: "/home/developer/webmux",
    cmd: [agent],
    status: pending ? "waiting" : chat.running ? "running" : "idle",
    // ACP permission requests feed the existing attention signal.
    attention: pending
      ? { kind: "permission", source: "agent", since: session.lastActiveAt }
      : null,
    preview: "",
    clients: 1,
    reportedCwd: null,
  });
  const terminal = {
    get info() {
      return info;
    },
    connection: "connected" as const,
    exitCode: null,
    restoring: false,
    historyTruncated: false,
    canFind: true,
    scrollToBottom() {},
    clear() {},
    resync() {},
  };
  const fullscreen = {
    offered: true,
    active: false,
    busy: false,
    toggle: async () => {},
  };

  function ask(title: string, body: string, run: () => void) {
    confirm = {
      title,
      body,
      run: () => {
        confirmOpen = false;
        run();
      },
    };
    confirmOpen = true;
  }
  function cancel() {
    chat.running = false;
    chat.timeline.push({
      type: "turnEnd",
      id: `end-${chat.timeline.length}`,
      stopReason: "cancelled",
    });
    chat.queuePaused = chat.queue.length > 0;
  }
  function advance() {
    if (!switching) return;
    if (switching.step + 1 < (switching.to === "tui" ? 3 : 4)) {
      switching.step++;
      timer = setTimeout(advance, 700);
      return;
    }
    chat.side = switching.to;
    switching = null;
  }
  function switchTo(to: Side) {
    if (to === chat.side || switching) return;
    const start = () => {
      if (chat.running && chat.side === "acp") cancel();
      switching = { to, step: 0 };
      timer = setTimeout(advance, 700);
    };
    if (chat.side === "acp" && chat.running)
      ask("切换到终端？", "当前回合会被中断，已输出的内容会保留。", start);
    else start();
  }
</script>

{#snippet quickMenu(side: "top" | "bottom")}
  <QuickCommandMenu
    commands={[]}
    target="terminal"
    unavailable=""
    disabled={false}
    {side}
    run={() => {}}
    manage={() => {}}
    onCloseAutoFocus={() => {}}
  />
{/snippet}

<main class="terminal-page">
  <TerminalHeader
    {terminal}
    {fullscreen}
    {draft}
    draftSaved="已保存在本机"
    scratchSaved="已保存在本机"
    uploading={false}
    desktop={desktopMedia.current}
    bind:codeShown
    forceSoftKeys={false}
    {quickMenu}
    upload={async () => {}}
    toggleDraft={() => {}}
    copy={async () => {}}
    paste={async () => {}}
    showScratch={() => {}}
    showFontSize={() => {}}
    showFind={() => {}}
    close={() => {}}
    hybrid={{ side: chat.side, switching: switching?.to ?? null, switchTo }}
  />
  {#if chat.side === "tui" && !switching && chat.previousSessionId && !idNoticeDismissed}
    <!-- D11, in the same strip TerminalNotices uses for page notices. -->
    <div
      class="flex shrink-0 items-center gap-2 border-b bg-muted px-4 py-1.5 text-sm"
      role="status"
    >
      <span class="min-w-0 flex-1"
        >TUI 已切换到新会话 <code class="font-mono"
          >{chat.agentSessionId.slice(0, 8)}…</code
        >（上一个
        <code class="font-mono">{chat.previousSessionId.slice(0, 8)}…</code
        >），切到聊天时接手新会话</span
      >
      <Button
        variant="ghost"
        size="sm"
        onclick={() => (idNoticeDismissed = true)}>知道了</Button
      >
    </div>
  {/if}
  <TerminalWorkspace
    {codeShown}
    desktop={desktopMedia.current}
    draftOpen={false}
    onfind={() => {}}
  >
    {#snippet terminalPane()}
      {#if switching}
        <SwitchProgress to={switching.to} step={switching.step} />
      {:else if chat.side === "acp"}
        <ChatView
          bind:chat
          onreview={() => (codeShown = true)}
          oncancel={cancel}
          onsendnow={(queued) =>
            ask("立即发送？", "当前回合会被中断，然后发送这条消息。", () => {
              const item = chat.queue.find((q) => q.id === queued);
              chat.queue = chat.queue.filter((q) => q.id !== queued);
              if (chat.running) cancel();
              if (item) {
                chat.timeline.push({
                  type: "user",
                  id: queued,
                  text: item.text,
                });
                chat.running = true;
              }
            })}
        />
      {:else}
        <!-- Same structure as TerminalViewport: xterm area plus the soft-key
             bar that touch devices show. -->
        <div class="terminal-column">
          <div class="terminal-area">
            <pre
              class="absolute inset-0 overflow-auto p-2 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap text-(--terminal-foreground)"
              aria-label="终端（示意）">{terminalLines}</pre>
          </div>
          <SoftKeys
            send={() => {}}
            modifiers={{ ctrl: false, alt: false }}
            toggle={() => {}}
            paste={() => {}}
            keyboard={() => {}}
          >
            {#snippet trailing()}{@render quickMenu("top")}{/snippet}
          </SoftKeys>
        </div>
      {/if}
    {/snippet}
    {#snippet draftPane()}{/snippet}
    {#snippet codePane()}
      <div class="h-full overflow-y-auto bg-card p-2">
        <CodeChangesList
          entries={codeChanges}
          total={codeChanges.length}
          nextCursor=""
          version="prototype"
          pending={false}
          enabled
          onmore={() => {}}
          onopen={() => {}}
        />
      </div>
    {/snippet}
  </TerminalWorkspace>
</main>

{#if confirm}
  {@const current = confirm}
  <ConfirmDialog
    bind:open={confirmOpen}
    title={current.title}
    onconfirm={current.run}>{current.body}</ConfirmDialog
  >
{/if}
