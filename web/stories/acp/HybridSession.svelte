<script lang="ts">
  import { paneEnter } from "../../src/hooks/shared/pane-enter";
  import { onDestroy, untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import type {
    ButtonSize,
    ButtonVariant,
  } from "../../src/lib/components/ui/button";
  import ChatView from "../../src/components/chat/ChatView.svelte";
  import SwitchChoiceDialog from "../../src/components/chat/SwitchChoiceDialog.svelte";
  import SwitchProgress from "../../src/components/chat/SwitchProgress.svelte";
  import CodeChangesList from "../../src/components/code/CodeChangesList.svelte";
  import { ListButton } from "$lib/components/ui/list-button";
  import { dockGroup, dockSplit } from "$lib/dock-layout";
  import DockPanel from "../../src/components/shared/DockPanel.svelte";
  import DockWorkspace from "../../src/components/shared/DockWorkspace.svelte";
  import ConfirmDialog from "../../src/components/shared/ConfirmDialog.svelte";
  import ConnectorInstallHint from "../../src/components/sessions/ConnectorInstallHint.svelte";
  import HybridCandidatePicker from "../../src/components/sessions/HybridCandidatePicker.svelte";
  import QuickCommandMenu from "../../src/components/settings/QuickCommandMenu.svelte";
  import SoftKeys from "../../src/components/terminal/SoftKeys.svelte";
  import TerminalHeader from "../../src/components/terminal/TerminalHeader.svelte";
  import TerminalSplitPanel from "../../src/components/terminal/TerminalSplitPanel.svelte";
  import TerminalNotices from "../../src/components/terminal/TerminalNotices.svelte";
  import * as chatFeature from "../../src/features/terminal-chat";
  import * as filesFeature from "../../src/features/terminal-files";
  import type { ChatActions } from "../../src/platform/chat-api";
  import type {
    HybridNotice,
    HybridView,
  } from "../../src/hooks/sessions/hybrid.svelte";
  import type {
    ChatUploads,
    UploadProgress,
  } from "../../src/hooks/shared/upload.svelte";
  import {
    eligibilityText,
    shortId,
    switchSteps,
    switchTarget,
    type SwitchTarget,
  } from "../../src/domains/sessions/hybrid";
  import type { ChatCodeLink } from "../../src/domains/code/markdown-link";
  import type {
    ChatAttachment,
    ChatState,
    SessionInfo,
    SessionView,
    TransitionPhase,
  } from "../../src/contracts/schemas";
  import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
  import { session } from "../fixtures";
  import {
    agentMetadata,
    chatFixture,
    claudeCandidates,
    codeChanges,
    terminalLines,
    withEvents,
    type Agent,
    type Scenario,
  } from "./fixtures";

  // The session page of a hybrid session, assembled from the page's own
  // components (header, notices, workspace split, chat view, switch progress
  // and dialogs) around sample chat frames. Commands change the local sample
  // state the way the server's frames would; nothing leaves the page.
  let {
    scenario = "conversation",
    agent = "omp",
    uploading = false,
  }: { scenario?: Scenario; agent?: Agent; uploading?: boolean } = $props();
  const fixture = untrack(() => chatFixture(scenario, agent));
  let model = $state.raw(fixture.model);
  let chat = $state.raw<ChatState>(fixture.state);
  // Where the session is, as its SessionInfo would say: the settled view and
  // a running switch's phase.
  let where = $state<SessionView>(fixture.view);
  let phase = $state<TransitionPhase | undefined>(fixture.phase);
  let confirming = $state<SwitchTarget | null>(null);
  let choiceOpen = $state(untrack(() => scenario === "awaiting-choice"));
  let candidateOpen = $state(untrack(() => scenario === "ambiguous"));
  let candidatePick = $state(claudeCandidates[0]!.agentSessionId);
  let connectorDismissed = $state(false);
  let codeShown = $state(false);
  let dock = $state<DockWorkspace>();
  let pickerOpen = $state(false);

  function openCode() {
    codeShown = true;
    dock?.show("code");
  }
  let openedLocation = $state<ChatCodeLink | null>(null);
  let notice = $state("");
  let dismissed = $state<string[]>([]);
  let timer: ReturnType<typeof setTimeout> | undefined;
  onDestroy(() => clearTimeout(timer));
  const compact = new MediaQuery("(max-width: 639px)");
  const drafts = createTerminalDrafts();
  const draft = drafts.get(session.id);
  if (untrack(() => scenario === "commands")) drafts.edit(draft, "/", 1);
  let uploadProgress = $state<UploadProgress | null>(
    untrack(() =>
      uploading
        ? {
            name: "screenshot.png",
            percent: 62,
            index: 1,
            total: 1,
            waiting: false,
          }
        : null,
    ),
  );
  const samplePreview =
    "data:image/svg+xml;utf8," +
    encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="64" viewBox="0 0 96 64"><rect width="96" height="64" rx="6" fill="#18221d"/><rect x="10" y="10" width="76" height="14" rx="3" fill="#2d6a4f"/><rect x="10" y="30" width="52" height="10" rx="2" fill="#40916c"/><rect x="10" y="46" width="64" height="8" rx="2" fill="#52b788"/></svg>`,
    );
  let attachments = $state<ChatAttachment[]>(
    untrack(() =>
      uploading
        ? [
            {
              path: "/tmp/webmux-upload-story/home-order.png",
              name: "home-order.png",
              mimeType: "image/png",
              size: 48_213,
              kind: "image",
            },
            {
              path: "/tmp/webmux-upload-story/notes.txt",
              name: "notes.txt",
              mimeType: "text/plain",
              size: 1_204,
              kind: "file",
            },
          ]
        : [],
    ),
  );
  const chatUploads: ChatUploads = {
    get progress() {
      return uploadProgress;
    },
    get attachments() {
      return attachments;
    },
    preview(path) {
      return path.endsWith(".png") ? samplePreview : undefined;
    },
    async uploadFiles(files) {
      attachments = [
        ...attachments,
        ...files.map((file, index) => ({
          path: `/tmp/webmux-upload-story/${attachments.length + index}-${file.name}`,
          name: file.name,
          mimeType: file.type || "text/plain",
          size: file.size,
          kind: "file" as const,
        })),
      ];
    },
    filePaste() {
      return false;
    },
    cancel() {
      uploadProgress = null;
    },
    remove(path) {
      attachments = attachments.filter((item) => item.path !== path);
    },
    take() {
      const taken = attachments;
      attachments = [];
      return taken;
    },
    commit() {},
    restore(sent) {
      attachments = [...sent, ...attachments];
    },
  };

  const side = $derived<SwitchTarget | null>(phase ? null : where);
  // The view before the current switch: chat unless it started on the TUI.
  let from = $state<SwitchTarget>(
    untrack(() => (scenario === "awaiting-choice" ? "terminal" : "chat")),
  );
  const view = $derived<HybridView>(
    side === null ? "switching" : side === "terminal" ? "terminal" : "chat",
  );
  const target = $derived(
    view === "switching" ? switchTarget(phase, from) : null,
  );
  const chatUnavailable = $derived(
    side === "terminal" ? eligibilityText(chat.eligibility) : "",
  );
  const info = $derived<SessionInfo>({
    ...session,
    title: model.title ?? `${agentMetadata[agent].title} · webmux`,
    cwd: "/home/developer/webmux",
    cmd: [agentMetadata[agent].executable],
    status: chat.requests.length
      ? "waiting"
      : chat.turn.running
        ? "running"
        : "idle",
    attention: chat.requests.length
      ? {
          kind: chat.requests[0]!.kind === "form" ? "question" : "permission",
          source: "agent",
          since: session.lastActiveAt,
        }
      : null,
    preview: "",
    clients: 1,
    reportedCwd: null,
    views: ["terminal", "chat"],
    view: where,
    transition: phase && target ? { to: target, phase } : null,
    agent: {
      name: agent,
      title: agentMetadata[agent].title,
      icon: agentMetadata[agent].icon,
      sessionId: fixture.agentSessionId,
    },
  });
  const notices = $derived.by(() => {
    const list: HybridNotice[] = [];
    if (chat.switchError)
      list.push({
        key: `failure:${chat.switchError}`,
        tone: "danger",
        text: chat.switchError,
      });
    if (notice)
      list.push({ key: `notice:${notice}`, tone: "info", text: notice });
    const change = chat.idChange;
    if (change && side === "terminal")
      list.push({
        key: `id:${change.from}>${change.to}`,
        tone: "info",
        text: `TUI 已切换到新会话 ${shortId(change.to)}（上一个：${shortId(change.from)}），切到聊天时接手新会话`,
      });
    return list.filter((item) => !dismissed.includes(item.key));
  });

  function update(patch: Partial<ChatState>) {
    chat = { ...chat, ...patch };
  }
  function interrupt() {
    model = withEvents(model, [
      {
        kind: "turn.end",
        turnEnd: { turnId: `turn-${model.next}`, stopReason: "cancelled" },
      },
    ]);
    update({
      turn: { running: false, stopReason: "cancelled" },
      requests: [],
      queue: { ...chat.queue, paused: chat.queue.items.length > 0 },
    });
  }
  const done = () => Promise.resolve();
  const actions: ChatActions = {
    send(text, _mode, sent) {
      if (chat.turn.running || chat.queue.paused) {
        const item = {
          id: `q${model.next}`,
          text,
          createdAt: session.lastActiveAt,
          ...(sent?.length ? { attachments: sent } : {}),
        };
        update({
          queue: { ...chat.queue, items: [...chat.queue.items, item] },
        });
      } else {
        model = withEvents(model, [
          {
            kind: "message.append",
            message: {
              messageId: `u${model.next}`,
              role: "user",
              text,
              ...(sent?.length ? { attachments: sent } : {}),
            },
          },
        ]);
        update({ turn: { running: true } });
      }
      return done();
    },
    cancel() {
      interrupt();
      return done();
    },
    editQueued(itemId, text) {
      update({
        queue: {
          ...chat.queue,
          items: chat.queue.items.map((item) =>
            item.id === itemId ? { ...item, text } : item,
          ),
        },
      });
      return done();
    },
    moveQueuedUp(itemId) {
      const items = [...chat.queue.items];
      const index = items.findIndex((item) => item.id === itemId);
      if (index > 0) {
        const moved = items.splice(index, 1)[0]!;
        items.splice(index - 1, 0, moved);
        update({ queue: { ...chat.queue, items } });
      }
      return done();
    },
    removeQueued(itemId) {
      const items = chat.queue.items.filter((item) => item.id !== itemId);
      update({ queue: { paused: chat.queue.paused && !!items.length, items } });
      return done();
    },
    sendQueuedNow: done,
    resumeQueue() {
      update({
        queue: { ...chat.queue, paused: false },
        turn: { running: true },
      });
      return done();
    },
    clearQueue() {
      update({ queue: { paused: false, items: [] } });
      return done();
    },
    answer(requestId, answer) {
      model = withEvents(model, [
        {
          kind: "request.close",
          resolved: {
            requestId,
            outcome: answer.outcome,
            ...("optionId" in answer ? { optionId: answer.optionId } : {}),
          },
        },
      ]);
      update({
        requests: chat.requests.filter((r) => r.requestId !== requestId),
      });
      return done();
    },
    setConfig(configId, value) {
      update({
        config: chat.config.map((option) =>
          option.id === configId ? { ...option, ...value } : option,
        ),
      });
      return done();
    },
    setPermissionMode(mode) {
      update({ permissionMode: mode });
      return done();
    },
  };

  // The phases a switch passes through, as the server reports them.
  const phases: Record<SwitchTarget, TransitionPhase[]> = {
    chat: ["interrupting", "stopping", "starting", "loading"],
    terminal: ["cancelling", "stopping", "starting"],
  };
  function advance(to: SwitchTarget, rest: TransitionPhase[]) {
    const [step, ...next] = rest;
    if (!step) {
      model = withEvents(model, [
        { kind: "marker", marker: { kind: "switched", view: to } },
      ]);
      where = to;
      phase = undefined;
      return;
    }
    phase = step;
    timer = setTimeout(() => advance(to, next), 700);
  }
  function run(to: SwitchTarget) {
    if (to === "terminal" && chat.turn.running) interrupt();
    from = to === "terminal" ? "chat" : "terminal";
    advance(to, phases[to]);
  }
  const hybrid = {
    get view() {
      return view;
    },
    get side() {
      return side;
    },
    get target() {
      return target;
    },
    get chatUnavailable() {
      return chatUnavailable;
    },
    request(to: SwitchTarget) {
      if (!side || to === side) return;
      notice = "";
      if (to === "chat" && chatUnavailable) {
        notice = `现在不能切到聊天：${chatUnavailable}`;
        return;
      }
      if (to === "terminal" && chat.turn.running) confirming = to;
      else run(to);
    },
  };
  const terminal = {
    get info() {
      return info;
    },
    connection: "connected" as const,
    exitCode: null,
    listeningPorts: [],
    canFind: true,
    clear() {},
    resync() {},
  };
  const noticeTerminal = {
    control: undefined,
    controlling: true,
    connection: "connected" as const,
    exitCode: null,
    listeningPorts: [],
    error: "",
    claim() {},
  };
  const fullscreen = {
    offered: true,
    active: false,
    busy: false,
    toggle: async () => {},
  };
  let bottomToolbarShown = $state(false);
</script>

{#snippet quickMenu(
  side: "top" | "bottom",
  size?: ButtonSize,
  variant?: ButtonVariant,
)}
  <QuickCommandMenu
    commands={[]}
    target="terminal"
    unavailable=""
    disabled={false}
    {side}
    {size}
    {variant}
    run={() => {}}
    manage={() => {}}
    onCloseAutoFocus={() => {}}
  />
{/snippet}

<main
  class="terminal-page"
  data-bottom-toolbar={bottomToolbarShown ? "shown" : "auto"}
>
  <TerminalHeader
    {pickerOpen}
    features={{ chat: chatFeature, files: filesFeature }}
    {terminal}
    {fullscreen}
    {draft}
    draftSaved="已保存在本机"
    scratchSaved="已保存在本机"
    uploading={false}
    sizeLocked={false}
    setSizeLocked={() => {}}
    keyboardSizeHold={true}
    setKeyboardSizeHold={() => {}}
    {codeShown}
    bind:bottomToolbarShown
    {quickMenu}
    upload={async () => {}}
    toggleDraft={() => {}}
    showScratch={() => {}}
    showFontSize={() => {}}
    showFind={() => {}}
    close={() => {}}
    leave={() => {}}
    {hybrid}
    openPanels={() => dock?.openPicker()}
  />
  <TerminalNotices
    features={{ chat: chatFeature, files: filesFeature }}
    terminal={noticeTerminal}
    uploads={{ progress: null, cancel() {} }}
    {draft}
    draftSaved="已保存在本机"
    error=""
    notice=""
    showScratch={() => {}}
    dismissScratch={() => {}}
    remove={() => {}}
    leave={() => {}}
    {notices}
    dismiss={(key) => {
      if (key.startsWith("notice:")) notice = "";
      else dismissed = [...dismissed, key];
    }}
  />
  {#if scenario === "connector-recheck" && !connectorDismissed && agentMetadata[agent].connector}
    <div class="shrink-0 border-b px-4 py-2">
      <ConnectorInstallHint
        connector={agentMetadata[agent].connector}
        detail="安装后会自动重新检测"
        ondismiss={() => (connectorDismissed = true)}
      />
    </div>
  {/if}
  <DockWorkspace
    bind:this={dock}
    storageKey="story.hybrid"
    initial={dockSplit("horizontal", dockGroup("session"), dockGroup("code"))}
    bind:showPicker={pickerOpen}
    compact={compact.current}
    pickerTrigger={false}
  >
    {#snippet pickerActions(closePanel)}
      {#if !codeShown}
        <ListButton
          variant="action-sheet"
          onclick={() => {
            openCode();
            closePanel();
          }}>代码</ListButton
        >
      {/if}
      <TerminalSplitPanel
        busy={false}
        limit={false}
        ready
        onsplit={() => {
          closePanel();
          notice = "预览：在真实会话中创建分屏。";
        }}
        oncreate={() => {
          closePanel();
          notice = "预览：在真实会话中打开新建会话。";
        }}
        onchoose={() => {
          closePanel();
          notice = "预览：在真实会话中选择已有会话。";
        }}
        onclose={() => {
          closePanel();
          notice = "预览：在真实会话中关闭当前窗格。";
        }}
      />
    {/snippet}
    <DockPanel id="session" title={view === "chat" ? "聊天" : "终端"}>
      {#if view === "chat"}
        <div class="h-full min-h-0" use:paneEnter={true}>
          <ChatView
            {model}
            {chat}
            connection="connected"
            {draft}
            {drafts}
            uploads={chatUploads}
            {actions}
            onreview={(path) => {
              openedLocation = { path };
              openCode();
            }}
            onopenlocation={(target) => {
              openedLocation = target;
              openCode();
            }}
            onterminal={() => hybrid.request("terminal")}
          />
        </div>
      {:else if view === "switching"}
        <SwitchProgress
          {target}
          steps={phase ? switchSteps(target, phase) : []}
          awaitingChoice={phase === "awaiting_choice"}
          onchoose={() => (choiceOpen = true)}
        />
      {:else}
        <!-- Same structure as TerminalViewport: xterm area plus the soft-key
             bar that touch devices show. -->
        <div class="terminal-column" use:paneEnter={true}>
          <div class="terminal-area">
            <pre
              class="absolute inset-0 overflow-auto p-2 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap text-(--terminal-foreground)"
              aria-label="终端（示意）">{terminalLines(agent)}</pre>
          </div>
          <SoftKeys
            send={() => {}}
            modifiers={{ ctrl: false, alt: false, shift: false }}
            toggle={() => {}}
            paste={() => {}}
            keyboard={() => {}}
          >
            {#snippet trailing()}{@render quickMenu(
                "top",
                "icon-xs",
                "ghost",
              )}{/snippet}
          </SoftKeys>
        </div>
      {/if}
    </DockPanel>
    {#if codeShown}
      <DockPanel id="code" title="代码工作区">
        <div class="h-full space-y-2 overflow-y-auto bg-card p-2">
          {#if openedLocation}
            <p
              class="rounded-md border bg-muted/40 px-2.5 py-1.5 font-mono text-xs break-all text-foreground"
              role="status"
            >
              已定位：{openedLocation.path}{openedLocation.line === undefined
                ? ""
                : `:${openedLocation.line}${openedLocation.endLine ? `-${openedLocation.endLine}` : ""}`}
            </p>
          {/if}
          <CodeChangesList
            entries={codeChanges}
            total={codeChanges.length}
            nextCursor=""
            version="story"
            pending={false}
            enabled
            onmore={() => {}}
            onopen={() => {}}
          />
        </div>
      </DockPanel>
    {/if}
  </DockWorkspace>
</main>

<ConfirmDialog
  bind:open={
    () => !!confirming,
    (open) => {
      if (!open) confirming = null;
    }
  }
  title="切换到终端？"
  onconfirm={() => {
    confirming = null;
    run("terminal");
  }}>当前回合会被中断，已输出的内容会保留。</ConfirmDialog
>
<ConfirmDialog
  bind:open={candidateOpen}
  title="选择要切到聊天的会话"
  onconfirm={() => {
    candidateOpen = false;
    run("chat");
  }}
  >无法确定当前会话，请选择要打开的会话。<HybridCandidatePicker
    candidates={claudeCandidates}
    bind:picked={candidatePick}
    label="选择要打开的会话"
  /></ConfirmDialog
>
<SwitchChoiceDialog
  bind:open={choiceOpen}
  note={chat.forceSwitchNote}
  busy={false}
  onchoose={(choice) => {
    choiceOpen = false;
    if (choice === "cancel") {
      where = "terminal";
      phase = undefined;
    } else advance("chat", phases.chat.slice(1));
  }}
/>
