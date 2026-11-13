<script lang="ts" module>
  import type { GitAmendDraftPrompt } from "../../hooks/workspace/git-commit-options.svelte";

  export interface GitCommitComposerCallbacks {
    onCommitMessageChange: (msg: string) => void;
    onOpenCommitPreview: (followUpPush: boolean) => void;
    onOpenPush?: () => void;
    onCreateBranchFromHead?: () => void;
    onResolveDraftKeepLocal?: () => void;
    onResolveDraftAdoptRemote?: () => void;
    onResolveDraftMergeBoth?: () => void;
    onRequestCommitAssist?: () => void;
    onToggleCommitAmend?: (checked: boolean) => void;
    onResolveAmendDraft?: (choice: "replace" | "merge" | "keep") => void;
    onCommitAuthorOverrideChange?: (value: string) => void;
    onToggleCommitSignoff?: (checked: boolean) => void;
  }
</script>

<script lang="ts">
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconCheck from "~icons/tabler/check";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconSparkles from "~icons/tabler/sparkles";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { GitWorktreeContext } from "../../contracts/git-workbench-schemas";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import { validateAuthorOverrideInput } from "../../domains/workspace/git-v2-adapter";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitCommitAdvancedOptions from "./GitCommitAdvancedOptions.svelte";
  import GitCommitDraftConflictCard from "./GitCommitDraftConflictCard.svelte";

  let {
    context,
    stagedCount,
    conflictedCount = 0,
    commitMessage,
    readonlyMode = false,
    busy = false,
    uncertainBlocked = false,
    draftSaving = false,
    draftRevision = 0,
    draftConflict = null,
    canAssistCommitDraft = false,
    commitAmend = false,
    commitAmendPushed = false,
    commitAuthorOverride = "",
    commitSignoff = false,
    commitSignoffTrailer = "",
    amendDraftPrompt = null,
    onCommitMessageChange,
    onOpenCommitPreview,
    onOpenPush,
    onCreateBranchFromHead,
    onResolveDraftKeepLocal,
    onResolveDraftAdoptRemote,
    onResolveDraftMergeBoth,
    onRequestCommitAssist,
    onToggleCommitAmend,
    onResolveAmendDraft,
    onCommitAuthorOverrideChange,
    onToggleCommitSignoff,
  }: GitCommitComposerCallbacks & {
    context: GitWorktreeContext;
    stagedCount: number;
    conflictedCount?: number;
    commitMessage: string;
    readonlyMode?: boolean;
    busy?: boolean;
    uncertainBlocked?: boolean;
    draftSaving?: boolean;
    draftRevision?: number;
    draftConflict?: GitCommitDraftConflictState | null;
    canAssistCommitDraft?: boolean;
    commitAmend?: boolean;
    commitAmendPushed?: boolean;
    commitAuthorOverride?: string;
    commitSignoff?: boolean;
    commitSignoffTrailer?: string;
    amendDraftPrompt?: GitAmendDraftPrompt | null;
  } = $props();

  const parsed = $derived.by(() => {
    const idx = commitMessage.indexOf("\n");
    if (idx === -1) return { subject: commitMessage, body: "" };
    const subject = commitMessage.slice(0, idx);
    const rest = commitMessage.slice(idx + 1).replace(/^\n/, "");
    return { subject, body: rest };
  });

  function updateSubject(nextSubject: string) {
    const clean = nextSubject.replace(/\r?\n/g, " ");
    if (!parsed.body) {
      onCommitMessageChange(clean);
    } else {
      onCommitMessageChange(`${clean}\n\n${parsed.body}`);
    }
  }

  function updateBody(nextBody: string) {
    if (!nextBody) {
      onCommitMessageChange(parsed.subject);
    } else {
      onCommitMessageChange(`${parsed.subject}\n\n${nextBody}`);
    }
  }

  const authorValidation = $derived(
    validateAuthorOverrideInput(commitAuthorOverride),
  );
  const identityMissing = $derived(
    !readonlyMode &&
      !commitAuthorOverride.trim() &&
      (!context.authorName.trim() || !context.authorEmail.trim()),
  );
  const detachedHead = $derived(context.headState === "detached");
  const unbornHead = $derived(context.headState === "unborn");
  const canPreviewCommit = $derived(
    !readonlyMode &&
      !busy &&
      !uncertainBlocked &&
      !draftConflict &&
      (stagedCount > 0 || (commitAmend && !unbornHead)) &&
      conflictedCount === 0 &&
      authorValidation.valid &&
      Boolean(parsed.subject.trim()),
  );
  const canDirectPush = $derived(
    !readonlyMode &&
      !uncertainBlocked &&
      !commitAmend &&
      stagedCount === 0 &&
      context.headState === "branch" &&
      Boolean(context.checkedOutBranch) &&
      (context.ahead > 0 || !context.upstream || Boolean(context.upstreamGone)),
  );

  const disabledHint = $derived.by(() => {
    if (readonlyMode) return "只读仓库模式无法创建提交。";
    if (draftConflict)
      return "存在跨设备提交草稿版本冲突：请先在上方冲突卡片选择保留、采用远端或合并后再预览提交。";
    if (uncertainBlocked)
      return "上次操作结果待核实（uncertain）：请先点击顶部提示中的「刷新历史与暂存区核实」，避免重复执行。";
    if (conflictedCount > 0)
      return "存在未解决冲突，请先在工作树终端解决全部冲突。";
    if (!authorValidation.valid) return authorValidation.error;
    if (commitAmend && unbornHead)
      return "当前工作树尚无提交（unborn HEAD），无法执行修正提交 (--amend)。";
    if (stagedCount === 0 && !commitAmend)
      return "暂存区为空：请先将变更加入“已暂存 (Staged)”后再提交，或开启修正上一次提交 (--amend)。";
    if (!parsed.subject.trim())
      return "请输入提交标题（Subject）后再预览提交。";
    return "";
  });

  function handleKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      if (canPreviewCommit) {
        // Safety invariant UI-10: Ctrl/Cmd+Enter ONLY opens preview, never direct-commits!
        onOpenCommitPreview(false);
      }
    }
  }
</script>

<div
  class="space-y-1.5 border-t border-git-border bg-git-panel p-2.5"
  data-testid="git-commit-composer"
>
  <div
    class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 text-xs text-muted-foreground"
  >
    <span class="min-w-0 truncate">
      {#if context.authorName && context.authorEmail}
        作者：{context.authorName} &lt;{context.authorEmail}&gt;
      {:else}
        作者身份：未配置（user.name / user.email）
      {/if}
    </span>

    <div class="flex flex-wrap items-center gap-1">
      {#if draftSaving}
        <Badge variant="outline">正在同步草稿…</Badge>
      {:else if draftRevision > 0 && commitMessage.trim()}
        <Badge variant="outline" data-testid="git-commit-draft-synced">
          <IconCheck class="size-3" aria-hidden="true" />
          草稿已同步 #{draftRevision}
        </Badge>
      {/if}
      {#if context.signEnabled}
        <Badge variant="info">
          {context.signFormat
            ? `${context.signFormat.toUpperCase()} 签名`
            : "提交签名开启"}
        </Badge>
      {/if}
      {#if context.executableHooks && context.executableHooks.length > 0}
        <Badge
          variant="outline"
          title={`可执行钩子：${context.executableHooks.join(", ")}`}
        >
          钩子 {context.executableHooks.length}
        </Badge>
      {/if}
    </div>
  </div>

  {#if draftConflict && onResolveDraftKeepLocal && onResolveDraftAdoptRemote && onResolveDraftMergeBoth}
    <GitCommitDraftConflictCard
      conflict={draftConflict}
      saving={draftSaving}
      onKeepLocal={onResolveDraftKeepLocal}
      onAdoptRemote={onResolveDraftAdoptRemote}
      onMergeBoth={onResolveDraftMergeBoth}
    />
  {/if}

  {#if detachedHead}
    <InlineNotice tone="warning">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span
          >当前处于游离 HEAD，建议先从当前 HEAD 新建并切换分支后再提交。</span
        >
        {#if onCreateBranchFromHead}
          <Button size="xs" variant="outline" onclick={onCreateBranchFromHead}>
            <IconGitBranch aria-hidden="true" />
            从当前 HEAD 新建分支…
          </Button>
        {/if}
      </div>
    </InlineNotice>
  {:else if identityMissing}
    <InlineNotice tone="warning">
      未检测到有效 Git 作者身份（user.name / user.email），提交前需在终端配置。
    </InlineNotice>
  {/if}

  <Input
    value={parsed.subject}
    placeholder="提交标题（Subject，必填 · Ctrl/Cmd+Enter 打开提交预览）"
    aria-label="提交说明标题"
    disabled={readonlyMode}
    oninput={(e) => updateSubject(e.currentTarget.value)}
    onkeydown={handleKeydown}
  />

  <Textarea
    rows={2}
    value={parsed.body}
    placeholder="补充提交正文描述（Body，选填）…"
    aria-label="补充正文描述"
    disabled={readonlyMode}
    oninput={(e) => updateBody(e.currentTarget.value)}
    onkeydown={handleKeydown}
  />

  {#if onToggleCommitAmend || onCommitAuthorOverrideChange || onToggleCommitSignoff}
    <GitCommitAdvancedOptions
      {context}
      {readonlyMode}
      amend={commitAmend}
      amendPushed={commitAmendPushed}
      authorOverride={commitAuthorOverride}
      signoff={commitSignoff}
      signoffTrailer={commitSignoffTrailer}
      draftPrompt={amendDraftPrompt}
      onToggleAmend={onToggleCommitAmend}
      {onResolveAmendDraft}
      onAuthorOverrideChange={onCommitAuthorOverrideChange}
      onToggleSignoff={onToggleCommitSignoff}
    />
  {/if}

  {#if disabledHint}
    <p class="text-xs text-muted-foreground" data-testid="git-commit-hint">
      {disabledHint}
    </p>
  {/if}

  <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
    <div class="min-w-0">
      {#if !readonlyMode && stagedCount > 0 && canAssistCommitDraft && onRequestCommitAssist}
        <Button
          size="xs"
          variant="ghost"
          onclick={onRequestCommitAssist}
          aria-label="让会话协助起草说明"
          title="将已暂存文件摘要发送到会话草稿框协助起草提交说明"
          data-testid="git-commit-assist-btn"
        >
          <IconSparkles aria-hidden="true" />
          <span class="sm:hidden">会话起草</span>
          <span class="max-sm:hidden">让会话协助起草说明</span>
        </Button>
      {/if}
    </div>

    <div class="flex flex-wrap items-center justify-end gap-2">
      {#if canDirectPush && onOpenPush}
        <Button variant="outline" onclick={onOpenPush}>
          <IconArrowUp aria-hidden="true" />
          推送待推提交…
        </Button>
      {/if}
      <Button
        variant="outline"
        disabled={!canPreviewCommit ||
          detachedHead ||
          (commitAmend && commitAmendPushed)}
        title={detachedHead
          ? "游离 HEAD 无法直接推送分支，请先新建分支"
          : commitAmend && commitAmendPushed
            ? "已推送提交修正后将导致分支与上游分叉，不支持一键普通推送（不提供强推）"
            : disabledHint || undefined}
        onclick={() => onOpenCommitPreview(true)}
        data-testid="git-commit-and-push-btn"
      >
        提交并推送…
      </Button>
      <Button
        variant="git-primary"
        disabled={!canPreviewCommit}
        title={disabledHint || undefined}
        onclick={() => onOpenCommitPreview(false)}
      >
        提交预览…
      </Button>
    </div>
  </div>
</div>
