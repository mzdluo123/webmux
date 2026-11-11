<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitCommitPreviewModel } from "../../contracts/git-workbench-schemas";
  import GitIssueList from "./GitIssueList.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    model,
    commitMessage,
    onConfirm,
    onClose,
  }: {
    model: GitCommitPreviewModel;
    commitMessage: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const canExecute = $derived(
    model.blockers.length === 0 && commitMessage.trim().length > 0,
  );
</script>

<ResponsiveDialog
  title={model.followUpPush
    ? `确认提交并进入推送预览 · ${model.repositoryName}`
    : `确认本地提交 · ${model.repositoryName}`}
  description="以下为真实 Git index（HEAD → index）的完整提交清单，包含工作台与终端预先暂存的全部条目。"
  drawerDescription="核对完整 Git index 暂存清单后确认提交。"
  truncateDrawerTitle
  class="sm:max-w-2xl"
  onclose={onClose}
>
  <div class="space-y-4">
    <div
      class="grid min-w-0 grid-cols-1 gap-1.5 rounded-lg border border-git-border bg-git-panel p-3 text-xs sm:grid-cols-2"
    >
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">目标分支：</span>
        <span class="font-medium text-foreground">{model.branch}</span>
      </p>
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">工作树：</span>
        <span class="font-mono text-foreground">{model.worktreePath}</span>
      </p>
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">作者身份：</span>
        <span class="text-foreground"
          >{model.authorName} &lt;{model.authorEmail}&gt;</span
        >
      </p>
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">签名与钩子：</span>
        <span class="text-foreground">
          {model.signEnabled ? "已启用签名" : "未启用签名"} ·
          {model.executableHooks.length > 0
            ? model.executableHooks.join(", ")
            : "无可执行钩子"}
        </span>
      </p>
    </div>

    {#if model.followUpPush}
      <InlineNotice tone="info">
        两阶段提交流程：第一步先执行本地提交并核对树对象一致性；仅当结果匹配预期（matched）时，第二步才会打开包含本分支全部待推提交的
        Push
        预览。若钩子改写了提交（needs_review）或结果待确认（uncertain），将自动停止推送。
      </InlineNotice>
    {/if}

    <GitIssueList blockers={model.blockers} warnings={model.warnings} />

    <div class="space-y-1.5">
      <div class="flex items-center justify-between">
        <span class="text-xs font-semibold text-foreground">
          即将写入本次提交的完整 index 条目 ({model.stagedFiles.length})
        </span>
        <span class="text-xs text-muted-foreground">
          未暂存剩余：{model.unstagedCount} · 未跟踪：{model.untrackedCount}
        </span>
      </div>

      <div
        class="max-h-48 space-y-1 overflow-y-auto rounded-lg border border-git-border bg-git-surface p-2"
      >
        {#if model.stagedFiles.length === 0}
          <p class="px-1 py-2 text-xs text-muted-foreground">
            当前 Git index 中没有已暂存条目。
          </p>
        {:else}
          {#each model.stagedFiles as file (file.id)}
            <div
              class="flex min-w-0 flex-wrap items-center gap-1.5 px-1 py-0.5 text-xs"
            >
              <Badge variant="info">{file.status}</Badge>
              <span class="min-w-0 flex-1 break-all font-mono">
                {file.path}
              </span>
              {#if file.preStagedFromCli}
                <Badge variant="warning">终端预暂存</Badge>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <div class="rounded-lg border border-git-border bg-git-panel p-2.5">
      <span class="text-xs font-semibold text-muted-foreground">提交说明：</span
      >
      <p
        class="mt-1 font-mono text-xs whitespace-pre-wrap wrap-anywhere text-foreground"
      >
        {commitMessage}
      </p>
    </div>

    <div class="flex flex-wrap items-center justify-end gap-2 pt-1">
      <Button variant="outline" onclick={onClose}>取消</Button>
      <Button variant="git-primary" disabled={!canExecute} onclick={onConfirm}>
        {model.followUpPush ? "确认提交并继续预览推送" : "确认本地提交"}
      </Button>
    </div>
  </div>
</ResponsiveDialog>
