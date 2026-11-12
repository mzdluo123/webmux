<script lang="ts">
  import type { FileDiffMetadata } from "@pierre/diffs";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { errorMessage } from "../../platform/error";
  import {
    gitChanges,
    gitContext,
    gitDiff,
    gitStagingDiff,
  } from "../../platform/git-api";
  import type {
    GitCommitPreviewModel,
    GitWorkingChange,
  } from "../../contracts/git-workbench-schemas";
  import {
    resolveCalculatedDiff,
    resolveHunksCalculatedDiff,
  } from "../../domains/code/code-diff-client";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    model,
    commitMessage,
    repositoryId = "",
    worktreeId,
    submitting = false,
    uncertainBlocked = false,
    onConfirm,
    onRefreshPreview,
    onCreateBranchFromHead,
    onClose,
  }: {
    model: GitCommitPreviewModel;
    commitMessage: string;
    repositoryId?: string;
    worktreeId?: string;
    submitting?: boolean;
    uncertainBlocked?: boolean;
    onConfirm: () => void;
    onRefreshPreview?: () => void;
    onCreateBranchFromHead?: () => void;
    onClose: () => void;
  } = $props();

  let expandedFileId = $state<string | null>(null);
  let inspectState = $state.raw<{
    phase: "idle" | "loading" | "ready" | "limited" | "error";
    fileDiff: FileDiffMetadata | null;
    reason: string;
  }>({
    phase: "idle",
    fileDiff: null,
    reason: "",
  });

  const expandedFile = $derived(
    expandedFileId
      ? (model.stagedFiles.find((f) => f.id === expandedFileId) ?? null)
      : null,
  );

  // Load real HEAD -> index diff when user expands any staged file in preview
  $effect(() => {
    const file: GitWorkingChange | null = expandedFile;
    if (!file) {
      inspectState = { phase: "idle", fileDiff: null, reason: "" };
      return;
    }
    if (file.binary) {
      inspectState = {
        phase: "limited",
        fileDiff: null,
        reason:
          file.degradeReason ||
          "二进制文件不展开文本行级差异（本次提交将写入该文件的暂存快照）。",
      };
      return;
    }
    const ctrl = new AbortController();
    inspectState = { phase: "loading", fileDiff: null, reason: "" };

    void (async () => {
      try {
        if (repositoryId && worktreeId) {
          const ctx = await gitContext(repositoryId, worktreeId, ctrl.signal);
          const list = await gitChanges(
            repositoryId,
            {
              contextId: ctx.contextId,
              worktreeId,
              group: "staged",
              q: file.path,
            },
            ctrl.signal,
          );
          const match = list.entries.find(
            (e) =>
              e.group === "staged" &&
              (e.newPath === file.path || e.oldPath === file.path),
          );
          if (match) {
            const raw = await gitDiff(
              repositoryId,
              {
                contextId: ctx.contextId,
                worktreeId,
                changeId: match.changeId,
              },
              ctrl.signal,
            );
            if (ctrl.signal.aborted) return;
            if (raw.kind === "text") {
              const calc = await resolveCalculatedDiff(raw, ctrl.signal);
              if (ctrl.signal.aborted) return;
              if (calc.calculation === "ready" && calc.fileDiff) {
                inspectState = {
                  phase: "ready",
                  fileDiff: calc.fileDiff,
                  reason: "",
                };
                return;
              }
            }
          }

          // Fallback via staging/diff endpoint when >200 changed files
          const stDiff = await gitStagingDiff(
            repositoryId,
            worktreeId,
            {
              area: "staged",
              path: file.path,
              ...(file.oldPath ? { oldPath: file.oldPath } : {}),
            },
            ctrl.signal,
          );
          if (ctrl.signal.aborted) return;
          if (stDiff.hunks.length > 0) {
            const calc = await resolveHunksCalculatedDiff(
              `commit-preview:${file.id}:${stDiff.hunks.length}`,
              file.path,
              file.oldPath ?? "",
              file.status,
              stDiff.hunks,
              ctrl.signal,
            );
            if (!ctrl.signal.aborted && calc.fileDiff) {
              inspectState = {
                phase: "ready",
                fileDiff: calc.fileDiff,
                reason: "",
              };
              return;
            }
          }
        }

        if (file.hunks.length > 0) {
          const calc = await resolveHunksCalculatedDiff(
            `commit-preview-fixture:${file.id}:${file.hunks.length}`,
            file.path,
            file.oldPath ?? "",
            file.status,
            file.hunks,
            ctrl.signal,
          );
          if (!ctrl.signal.aborted && calc.fileDiff) {
            inspectState = {
              phase: "ready",
              fileDiff: calc.fileDiff,
              reason: "",
            };
            return;
          }
        }

        if (!ctrl.signal.aborted) {
          inspectState = {
            phase: "limited",
            fileDiff: null,
            reason:
              file.degradeReason ||
              "该暂存条目仅包含模式、重命名或空内容变更。",
          };
        }
      } catch (err) {
        if (ctrl.signal.aborted) return;
        inspectState = {
          phase: "error",
          fileDiff: null,
          reason: errorMessage(err),
        };
      }
    })();

    return () => ctrl.abort();
  });

  const canExecute = $derived(
    !submitting &&
      !uncertainBlocked &&
      !model.stale &&
      model.blockers.length === 0 &&
      model.stagedFiles.length > 0 &&
      commitMessage.trim().length > 0,
  );

  function toggleInspectFile(id: string) {
    expandedFileId = expandedFileId === id ? null : id;
  }
</script>

<ResponsiveDialog
  title={model.followUpPush
    ? `确认提交并进入推送预览 · ${model.repositoryName}`
    : `确认本地提交 · ${model.repositoryName}`}
  description="以下为真实 Git index（HEAD → index）的完整提交清单，包含工作台与终端预先暂存的全部条目。"
  drawerDescription="核对完整 Git index 暂存清单后确认提交。"
  truncateDrawerTitle
  busy={submitting}
  class="sm:max-w-3xl"
  onclose={onClose}
>
  <div class="space-y-3">
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
        <span class="text-foreground">
          {model.authorName || "(未配置)"} &lt;{model.authorEmail ||
            "未配置"}&gt;
        </span>
      </p>
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">签名与钩子：</span>
        <span class="text-foreground">
          {model.signEnabled
            ? model.signFormat
              ? `已启用 ${model.signFormat.toUpperCase()} 签名`
              : "已启用签名"
            : "未启用签名"} ·
          {model.executableHooks.length > 0
            ? model.executableHooks.join(", ")
            : "无可执行钩子"}
        </span>
      </p>
    </div>

    {#if model.headState === "detached"}
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>
            当前工作树处于游离 HEAD；建议先从当前 HEAD 新建并切换分支后再提交。
          </span>
          {#if onCreateBranchFromHead}
            <Button
              size="xs"
              variant="outline"
              onclick={() => {
                onClose();
                onCreateBranchFromHead();
              }}
            >
              <IconGitBranch aria-hidden="true" />
              从当前 HEAD 新建并切换分支…
            </Button>
          {/if}
        </div>
      </InlineNotice>
    {/if}

    {#if model.stale}
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>
            {model.dialogError ||
              "暂存区或 HEAD 在预览打开期间已变化，当前确认令牌已失效；提交说明已保留。"}
          </span>
          {#if onRefreshPreview}
            <Button
              size="xs"
              variant="git-primary"
              disabled={submitting}
              onclick={onRefreshPreview}
            >
              <IconRefresh aria-hidden="true" />
              重新生成提交预览
            </Button>
          {/if}
        </div>
      </InlineNotice>
    {:else if model.dialogError}
      <InlineNotice tone="danger">{model.dialogError}</InlineNotice>
    {/if}

    {#if model.followUpPush}
      <InlineNotice tone="info">
        两阶段独立提交流程：第一步先执行本地提交并核对树对象；仅当结果完全匹配预期（matched）时才会进入第二步推送确认。若钩子改写了暂存区（needs_review）或状态待核实（uncertain），将自动中止推送。
      </InlineNotice>
    {/if}

    <GitIssueList blockers={model.blockers} warnings={model.warnings} />

    <div class="space-y-1.5">
      <div class="flex flex-wrap items-center justify-between gap-1">
        <span class="text-xs font-semibold text-foreground">
          即将写入本次提交的完整 HEAD → index 条目 ({model.stagedFiles.length})
        </span>
        <span class="text-xs text-muted-foreground">
          未暂存排除：{model.unstagedCount} · 未跟踪排除：{model.untrackedCount}
        </span>
      </div>

      <div
        class="max-h-44 space-y-0.5 overflow-y-auto rounded-lg border border-git-border bg-git-surface p-1.5"
      >
        {#if model.stagedFiles.length === 0}
          <p class="px-2 py-2 text-xs text-muted-foreground">
            当前 Git index 中没有已暂存条目。
          </p>
        {:else}
          {#each model.stagedFiles as file (file.id)}
            {@const isExpanded = expandedFileId === file.id}
            <ListButton
              variant="git-row"
              aria-expanded={isExpanded}
              aria-selected={isExpanded}
              onclick={() => toggleInspectFile(file.id)}
            >
              {#if isExpanded}
                <IconChevronDown class="size-3.5 shrink-0" aria-hidden="true" />
              {:else}
                <IconChevronRight
                  class="size-3.5 shrink-0"
                  aria-hidden="true"
                />
              {/if}
              <Badge variant="info">{file.status}</Badge>
              <span class="min-w-0 flex-1 truncate font-mono" title={file.path}>
                {file.oldPath && file.oldPath !== file.path
                  ? `${file.oldPath} → ${file.path}`
                  : file.path}
              </span>
              {#if file.preStagedFromCli}
                <Badge variant="warning">终端预暂存</Badge>
              {/if}
            </ListButton>
          {/each}
        {/if}
      </div>

      {#if expandedFile}
        <div
          class="code-panel flex h-52 min-w-0 flex-col overflow-hidden rounded-lg border border-git-border bg-git-surface"
          data-testid="git-commit-preview-diff"
        >
          {#if inspectState.phase === "loading"}
            <p class="p-3 text-xs text-muted-foreground">
              正在加载 HEAD → index 差异（{expandedFile.path}）…
            </p>
          {:else if inspectState.phase === "ready" && inspectState.fileDiff}
            <CodeDiffReader
              fileDiff={inspectState.fileDiff}
              side="new"
              expired={false}
              draftEnabled={false}
              preferredDiffStyle="unified"
              onselect={() => {}}
            />
          {:else}
            <div class="p-3">
              <InlineNotice
                tone={inspectState.phase === "error" ? "danger" : "info"}
              >
                {inspectState.reason}
              </InlineNotice>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <div class="rounded-lg border border-git-border bg-git-panel p-2.5">
      <span class="text-xs font-semibold text-muted-foreground">提交说明：</span
      >
      <p
        class="mt-1 max-h-24 overflow-y-auto font-mono text-xs whitespace-pre-wrap wrap-anywhere text-foreground"
      >
        {commitMessage}
      </p>
    </div>

    <div class="flex flex-wrap items-center justify-end gap-2 pt-1">
      <Button variant="outline" disabled={submitting} onclick={onClose}>
        取消
      </Button>
      <Button variant="git-primary" disabled={!canExecute} onclick={onConfirm}>
        {#if submitting}
          正在提交…
        {:else}
          {model.followUpPush ? "确认提交并继续预览推送" : "确认本地提交"}
        {/if}
      </Button>
    </div>
  </div>
</ResponsiveDialog>
