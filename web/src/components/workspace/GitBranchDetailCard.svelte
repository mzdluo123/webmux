<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    resolveGitBranchActions,
    resolveGitBranchUpstream,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    activeBranch,
    context,
    branchFilter = null,
    onRouteOp,
  }: {
    activeBranch: GitBranchItem | null;
    context: GitWorktreeContext;
    branchFilter?: string | null;
    onRouteOp: (kind: GitSharedBranchActionKind, item: GitBranchItem) => void;
  } = $props();

  const activeUpstream = $derived(
    activeBranch ? resolveGitBranchUpstream(activeBranch) : null,
  );
  const activeActions = $derived(
    activeBranch
      ? resolveGitBranchActions({
          item: activeBranch,
          context,
          branchFilter,
          compareBaseOid: context.headOid ?? null,
          compareTargetLabel: "与当前 HEAD 比较",
        })
      : [],
  );
</script>

{#if !activeBranch}
  <p class="text-xs text-muted-foreground">
    选择任意分支或标签查看详情、上游关联、工作树占用与可执行操作。
  </p>
{:else}
  {@const isCurrent =
    activeBranch.kind === "local" &&
    activeBranch.name === context.checkedOutBranch}
  {@const pushAct = activeActions.find((a) => a.kind === "push")}
  {@const ffAct = activeActions.find((a) => a.kind === "fast-forward")}
  {@const delAct = activeActions.find((a) => a.kind === "delete")}
  {@const renameAct = activeActions.find((a) => a.kind === "rename-branch")}
  {@const upstreamAct = activeActions.find((a) => a.kind === "set-upstream")}
  {@const createTagAct = activeActions.find((a) => a.kind === "create-tag")}
  {@const deleteTagAct = activeActions.find((a) => a.kind === "delete-tag")}
  <div class="space-y-3" data-testid="git-branch-detail-card">
    <div>
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <h3 class="font-mono text-sm font-semibold break-all text-foreground">
          {activeBranch.name}
        </h3>
        <Badge variant="outline">
          {#if activeBranch.kind === "tag"}
            {activeBranch.tagKind === "annotated" ? "附注标签" : "轻量标签"}
          {:else if activeBranch.kind === "local"}
            本地分支
          {:else}
            远程引用
          {/if}
        </Badge>
        {#if activeBranch.kind === "tag" && activeBranch.tagSigned}
          <Badge variant="info">已签名</Badge>
        {/if}
      </div>
      <p class="mt-1 text-xs wrap-anywhere text-muted-foreground">
        指向提交：{activeBranch.shortHead} · {activeBranch.subject}
      </p>
    </div>

    {#if activeBranch.kind === "tag"}
      <div
        class="space-y-1.5 rounded-md border border-git-border bg-git-surface p-2.5 text-xs"
      >
        <p class="font-mono break-all text-muted-foreground">
          目标提交 OID：{activeBranch.head}
        </p>
        {#if activeBranch.tagOid}
          <p class="font-mono break-all text-muted-foreground">
            标签对象 OID：{activeBranch.tagOid}
          </p>
        {/if}
        {#if activeBranch.taggerName}
          <p class="wrap-anywhere text-muted-foreground">
            标签者：{activeBranch.taggerName}
            {#if activeBranch.taggerEmail}
              &lt;{activeBranch.taggerEmail}&gt;
            {/if}
            {#if activeBranch.taggedAt}
              · {activeBranch.taggedAt}
            {/if}
          </p>
        {/if}
        {#if activeBranch.tagMessage}
          <p class="pt-1 whitespace-pre-wrap wrap-anywhere text-foreground">
            {activeBranch.tagMessage}
          </p>
        {/if}
      </div>
    {:else if activeBranch.kind === "local" && activeUpstream}
      <div
        class="rounded-md border border-git-border bg-git-surface p-2.5 text-xs"
      >
        <p class="wrap-anywhere">{activeUpstream.detailText}</p>
      </div>
    {/if}

    {#if activeBranch.occupiedBy}
      <InlineNotice tone="warning">
        该分支已在另一工作树
        <span class="font-mono break-all"
          >{activeBranch.occupiedBy.worktreePath}</span
        >
        检出（当前活跃会话：{activeBranch.occupiedBy.activeSessions} 个）。Git 不允许两个工作树同时检出同一本地分支，请直接前往该工作树或从该提交新建工作树分支。
      </InlineNotice>
    {/if}

    <div class="flex flex-wrap items-center gap-2 pt-1">
      <Button
        size="sm"
        variant="outline"
        onclick={() => onRouteOp("filter-log", activeBranch)}
      >
        在历史中查看提交
      </Button>

      {#if activeBranch.occupiedBy}
        <Button
          size="sm"
          variant="git-primary"
          onclick={() => onRouteOp("open-occupied-worktree", activeBranch)}
        >
          打开所在工作树
        </Button>
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("open-new-worktree", activeBranch)}
        >
          从该提交新建分支工作树…
        </Button>
      {:else if isCurrent && pushAct}
        <Button
          size="sm"
          variant="git-primary"
          disabled={pushAct.disabled}
          title={pushAct.disabledReason}
          onclick={() => onRouteOp("push", activeBranch)}
        >
          {pushAct.label}
        </Button>
      {:else if activeBranch.kind !== "tag" && context.worktreePath}
        <Button
          size="sm"
          variant="git-primary"
          onclick={() => onRouteOp("checkout", activeBranch)}
        >
          {activeBranch.kind === "remote" ? "检出并跟踪…" : "检出到当前工作树…"}
        </Button>
      {/if}

      {#if !activeBranch.occupiedBy && activeBranch.kind !== "tag"}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("create-branch-from", activeBranch)}
        >
          从此分支新建分支…
        </Button>
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("open-new-worktree", activeBranch)}
        >
          在新工作树打开…
        </Button>
      {/if}

      {#if activeBranch.kind === "local"}
        {#if renameAct}
          <Button
            size="sm"
            variant="outline"
            disabled={renameAct.disabled}
            title={renameAct.disabledReason}
            onclick={() => onRouteOp("rename-branch", activeBranch)}
            data-testid="git-branch-detail-rename-btn"
          >
            重命名…
          </Button>
        {/if}
        {#if upstreamAct}
          <Button
            size="sm"
            variant="outline"
            onclick={() => onRouteOp("set-upstream", activeBranch)}
            data-testid="git-branch-detail-upstream-btn"
          >
            设置上游…
          </Button>
        {/if}
        {#if ffAct}
          <Button
            size="sm"
            variant="outline"
            disabled={ffAct.disabled}
            title={ffAct.disabledReason}
            onclick={() => onRouteOp("fast-forward", activeBranch)}
          >
            {ffAct.label}
          </Button>
        {/if}
      {/if}

      {#if createTagAct}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("create-tag", activeBranch)}
          data-testid="git-branch-detail-create-tag-btn"
        >
          新建标签…
        </Button>
      {/if}

      {#if activeBranch.kind === "tag" && deleteTagAct}
        <Button
          size="sm"
          variant="destructive"
          onclick={() => onRouteOp("delete-tag", activeBranch)}
          data-testid="git-branch-detail-delete-tag-btn"
        >
          删除本地标签…
        </Button>
      {/if}

      {#if activeBranch.kind === "local" && delAct}
        <Button
          size="sm"
          variant="destructive"
          disabled={delAct.disabled}
          title={delAct.disabledReason}
          onclick={() => onRouteOp("delete", activeBranch)}
        >
          {delAct.label}
        </Button>
      {/if}
    </div>
  </div>
{/if}
