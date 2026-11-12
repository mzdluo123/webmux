<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCopy from "~icons/tabler/copy";
  import IconMessageForward from "~icons/tabler/message-forward";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as RadioGroup from "$lib/components/ui/radio-group";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
  import { formatLineRangeLabel } from "../../domains/workspace/git-rework-model";
  import type { GitSessionHandoffMode } from "../../hooks/workspace/git-rework-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    mode = "rework",
    items,
    stagedChanges = [],
    activeSessions,
    selectedSessionId = $bindable(""),
    checkingFreshness = false,
    stalePaths = [],
    staleConfirmed = false,
    copiedFeedback = false,
    hasCommitMessageDraft = false,
    onUpdateNote,
    onRemoveItem,
    onClearAll,
    onCopyMarkdown,
    onConfirmStale,
    onDeliverToSession,
    onStartNewSessionWithDraft,
    onClose,
  }: {
    mode?: GitSessionHandoffMode;
    items: readonly GitReworkItem[];
    stagedChanges?: readonly GitWorkingChange[];
    activeSessions: readonly SessionInfo[];
    selectedSessionId?: string;
    checkingFreshness?: boolean;
    stalePaths?: readonly string[];
    staleConfirmed?: boolean;
    copiedFeedback?: boolean;
    hasCommitMessageDraft?: boolean;
    onUpdateNote: (itemId: string, note: string) => void;
    onRemoveItem: (itemId: string) => void;
    onClearAll: () => void;
    onCopyMarkdown: () => void;
    onConfirmStale: () => void;
    onDeliverToSession: (session: SessionInfo) => void;
    onStartNewSessionWithDraft: () => void;
    onClose: () => void;
  } = $props();

  const isCommitAssist = $derived(mode === "commit-assist");
  const dialogTitle = $derived(
    isCommitAssist
      ? "让会话协助起草提交说明"
      : `返工意见草稿箱 (${items.length})`,
  );
  const dialogDescription = $derived(
    isCommitAssist
      ? "将当前真实 Git index 已暂存文件摘要发送到所选会话草稿框，不会自动执行命令。"
      : "汇总跨文件差异摘录与修改要求，安全放入目标工作树会话输入草稿，不会自动在终端执行。",
  );

  const chosenSession = $derived(
    activeSessions.find((s) => s.id === selectedSessionId),
  );
  const canSubmitPayload = $derived(
    isCommitAssist ? stagedChanges.length > 0 : items.length > 0,
  );
  const canSendToSession = $derived(
    Boolean(chosenSession) && canSubmitPayload && !checkingFreshness,
  );

  function sessionTitle(session: SessionInfo): string {
    return session.title.trim() || session.cmd.join(" ") || session.id;
  }

  function sessionViewLabel(session: SessionInfo): string {
    return session.view === "chat" ? "聊天视图" : "终端 TUI";
  }

  function sessionStatusLabel(status: SessionInfo["status"]): string {
    switch (status) {
      case "running":
        return "运行中";
      case "waiting":
        return "等待输入";
      case "idle":
        return "空闲";
      case "exited":
        return "已退出";
    }
  }
</script>

<ResponsiveDialog
  title={dialogTitle}
  description={dialogDescription}
  drawerDescription={isCommitAssist
    ? "将已暂存摘要放入会话草稿框。"
    : "汇总差异摘录并放入目标会话草稿框。"}
  pinDrawerHeader
  truncateDrawerTitle
  busy={checkingFreshness}
  class="sm:max-w-2xl"
  onclose={onClose}
>
  <div class="min-w-0 space-y-4 text-xs" data-testid="git-rework-dialog">
    {#if hasCommitMessageDraft && !isCommitAssist}
      <InlineNotice tone="info">
        当前正在编写的提交说明草稿已独立保留；编辑或发送返工意见不会覆盖或清空提交草稿。
      </InlineNotice>
    {/if}

    {#if isCommitAssist}
      <div
        class="space-y-2 rounded-md border border-git-border bg-git-panel p-3"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="font-semibold text-foreground">
            已暂存文件摘要（{stagedChanges.length} 项）
          </span>
          <Badge variant="info">Git index</Badge>
        </div>
        {#if stagedChanges.length === 0}
          <p class="text-muted-foreground">
            当前暂存区为空，请先暂存变更后再请求协助起草。
          </p>
        {:else}
          <ul class="max-h-40 space-y-1 overflow-y-auto font-mono">
            {#each stagedChanges as item (item.id)}
              <li class="truncate text-foreground">
                [{item.status}] {item.oldPath && item.oldPath !== item.path
                  ? `${item.oldPath} → ${item.path}`
                  : item.path}
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {:else if items.length === 0}
      <div class="rounded-md border border-git-border bg-git-panel p-4">
        <p class="text-muted-foreground">
          草稿箱暂无审阅意见。可在「提交」差异、「相对任务起点的变化」差异或「日志」历史差异中点击行号选区，或点击代码块上的「加入意见」添加。
        </p>
      </div>
    {:else}
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <span class="font-semibold text-foreground">
            待交回审阅条目（共 {items.length} 条）
          </span>
          <Button size="xs" variant="ghost" onclick={onClearAll}>
            <IconTrash aria-hidden="true" />
            清空全部
          </Button>
        </div>

        <div class="max-h-72 space-y-2.5 overflow-y-auto pr-0.5">
          {#each items as item, idx (item.id)}
            <div
              class="space-y-2 rounded-md border border-git-border bg-git-panel p-2.5"
              data-testid="git-rework-item"
              data-rework-path={item.path}
            >
              <div
                class="flex min-w-0 flex-wrap items-center justify-between gap-1.5"
              >
                <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                  <Badge variant="outline">#{idx + 1}</Badge>
                  <span
                    class="min-w-0 truncate font-mono font-medium text-foreground"
                    title={item.path}
                  >
                    {item.path}
                  </span>
                  <Badge
                    variant={item.sourceKind === "worktree"
                      ? "info"
                      : "secondary"}
                    class="max-w-full"
                  >
                    <span class="truncate">{item.versionLabel}</span>
                  </Badge>
                  <Badge variant="outline">
                    <span class="font-mono">
                      {formatLineRangeLabel(item.startLine, item.endLine)}
                    </span>
                  </Badge>
                </div>

                <Button
                  size="icon-xs"
                  variant="ghost"
                  aria-label={`移除审阅意见 ${item.path}`}
                  title={`移除审阅意见 ${item.path}`}
                  onclick={() => onRemoveItem(item.id)}
                >
                  <IconTrash aria-hidden="true" />
                </Button>
              </div>

              {#if item.codeSnippet.trim()}
                <pre
                  class="max-h-28 overflow-y-auto rounded-sm border border-git-border bg-git-surface p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all text-foreground">{item.codeSnippet}</pre>
              {/if}

              <Textarea
                rows={2}
                value={item.userNote}
                placeholder="填写具体返工修改说明（留空将使用默认提示）…"
                aria-label={`第 ${idx + 1} 条审阅意见说明`}
                oninput={(e) => onUpdateNote(item.id, e.currentTarget.value)}
              />
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if stalePaths.length > 0 && !staleConfirmed}
      <InlineNotice tone="warning">
        <div
          class="flex flex-wrap items-center justify-between gap-2"
          data-testid="git-rework-stale-warning"
        >
          <span>
            检测到工作区文件自摘录后已有新改动（{stalePaths.join(
              "、",
            )}），实时行号可能已偏移。生成文本将自动附加偏移提醒，请确认是否继续发送。
          </span>
          <Button
            size="xs"
            variant="outline"
            onclick={onConfirmStale}
            data-testid="git-rework-confirm-stale"
          >
            确认文件已更新仍可发送
          </Button>
        </div>
      </InlineNotice>
    {/if}

    <section class="min-w-0 space-y-2 border-t border-git-border pt-3">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <span class="font-semibold text-foreground">目标工作树接收会话</span>
        {#if activeSessions.length >= 2}
          <Badge variant="warning" class="max-w-full">
            <span class="truncate">必须显式选择会话</span>
          </Badge>
        {/if}
      </div>

      {#if activeSessions.length === 0}
        <InlineNotice tone="info">
          当前工作树暂无活动会话。你可以复制结构化
          Markdown，或直接在该工作树新建会话并带入内容。
        </InlineNotice>
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={!canSubmitPayload}
            onclick={onCopyMarkdown}
            aria-label="复制结构化审阅意见 Markdown"
            title="复制结构化审阅意见 Markdown"
            data-testid="git-rework-copy-markdown"
          >
            {#if copiedFeedback}
              <IconCheck aria-hidden="true" />
              <span>已复制</span>
            {:else}
              <IconCopy aria-hidden="true" />
              <span class="sm:hidden">复制意见</span>
              <span class="max-sm:hidden">复制结构化审阅意见 Markdown</span>
            {/if}
          </Button>
          <Button
            size="sm"
            variant="git-primary"
            disabled={!canSubmitPayload}
            onclick={onStartNewSessionWithDraft}
            aria-label="在该工作树新建会话并带入意见"
            title="在该工作树新建会话并带入意见"
            data-testid="git-rework-new-session"
          >
            <IconPlus aria-hidden="true" />
            <span class="sm:hidden">新建会话带入</span>
            <span class="max-sm:hidden">在该工作树新建会话并带入意见</span>
          </Button>
        </div>
      {:else if activeSessions.length === 1}
        {@const single = activeSessions[0]!}
        <div
          class="flex min-w-0 flex-wrap items-center justify-between gap-2 rounded-md border border-foreground/40 bg-git-panel px-3 py-2.5"
          data-testid="git-rework-single-session"
        >
          <div class="min-w-0 space-y-0.5">
            <p class="truncate text-sm font-medium text-foreground">
              {sessionTitle(single)}
            </p>
            <p class="text-xs text-muted-foreground">
              将写入该会话输入草稿框，由你在会话中核对后发送，绝不会在终端自动回车执行。
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-1.5">
            <Badge variant="info">{sessionViewLabel(single)}</Badge>
            <Badge variant="outline">{sessionStatusLabel(single.status)}</Badge>
          </div>
        </div>
      {:else}
        <p class="text-muted-foreground">
          当前工作树有 {activeSessions.length} 个活动会话，请选择要接收草稿的具体会话（不会默认盲选）：
        </p>
        <RadioGroup.Root
          bind:value={selectedSessionId}
          aria-label="选择目标接收会话"
          data-testid="git-rework-session-picker"
        >
          {#each activeSessions as session (session.id)}
            <RadioGroup.Item
              value={session.id}
              aria-label={sessionTitle(session)}
              title={sessionTitle(session)}
              description={`${sessionViewLabel(session)} · ${sessionStatusLabel(session.status)} · ID ${session.id}`}
            />
          {/each}
        </RadioGroup.Root>
      {/if}
    </section>

    {#if activeSessions.length > 0}
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-t border-git-border pt-3"
      >
        <Button
          size="sm"
          variant="outline"
          disabled={!canSubmitPayload}
          onclick={onCopyMarkdown}
          data-testid="git-rework-copy-markdown"
        >
          {#if copiedFeedback}
            <IconCheck aria-hidden="true" />
            已复制
          {:else}
            <IconCopy aria-hidden="true" />
            复制 Markdown
          {/if}
        </Button>

        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <Button size="sm" variant="ghost" onclick={onClose}>关闭</Button>
          <Button
            size="sm"
            variant="git-primary"
            disabled={!canSendToSession}
            onclick={() => chosenSession && onDeliverToSession(chosenSession)}
            aria-label="发送到所选会话草稿"
            title="发送到所选会话草稿"
            data-testid="git-rework-send-session"
          >
            <IconMessageForward aria-hidden="true" />
            {#if checkingFreshness}
              <span>校验中…</span>
            {:else}
              <span class="sm:hidden">发送到会话</span>
              <span class="max-sm:hidden">发送到所选会话草稿</span>
            {/if}
          </Button>
        </div>
      </div>
    {/if}
  </div>
</ResponsiveDialog>
