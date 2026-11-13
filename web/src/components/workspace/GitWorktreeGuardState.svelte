<script lang="ts">
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconHistory from "~icons/tabler/history";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";

  let {
    kind,
    repositories = [],
    repository,
    worktree,
    canAddRepository = true,
    onAddRepository,
    onSelectRepository,
    onSelectWorktree,
    onNewWorktree,
    onManageWorktrees,
    onBackToLog,
  }: {
    kind:
      | "empty-registry"
      | "missing-repository"
      | "unavailable-repository"
      | "pick-worktree"
      | "broken-worktree";
    repositories?: readonly Repository[];
    repository?: Repository;
    worktree?: Worktree;
    canAddRepository?: boolean;
    onAddRepository: () => void;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
    onBackToLog?: () => void;
  } = $props();

  const writableWorktrees = $derived(
    (repository?.worktrees ?? []).filter(
      (wt) => wt.available && !wt.prunable && !wt.locked && wt.kind !== "bare",
    ),
  );
</script>

{#snippet writableList(showDirty: boolean)}
  <div class="space-y-1">
    {#each writableWorktrees as wt (wt.id)}
      <ListButton variant="git-row" onclick={() => onSelectWorktree(wt.id)}>
        <span
          class={`min-w-0 flex-1 truncate ${showDirty ? "font-medium" : ""}`}
        >
          {worktreeTitle(wt)}
        </span>
        {#if showDirty && wt.dirty}
          <Badge variant="warning">有改动</Badge>
        {/if}
        <span class="max-w-48 truncate font-mono text-xs text-muted-foreground">
          {wt.path}
        </span>
      </ListButton>
    {/each}
  </div>
{/snippet}

<div
  class="flex h-full min-h-0 flex-1 items-center justify-center overflow-y-auto p-4 sm:p-8"
  data-testid={`git-guard-${kind}`}
>
  <div
    class="w-full max-w-lg space-y-4 rounded-lg border border-git-border bg-git-panel p-5 shadow-xs"
  >
    {#if kind === "empty-registry"}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <IconFolderPlus
            class="size-5 text-muted-foreground"
            aria-hidden="true"
          />
          <h2>尚未登记任何 Git 仓库</h2>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          Git
          工作台无需启动终端会话即可浏览提交历史、管理分支与暂存提交。请先登记一个本地
          Git 仓库。
        </p>
      </div>
      {#if canAddRepository}
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="git-primary" onclick={onAddRepository}>
            <IconPlus aria-hidden="true" />
            <span>登记已有仓库…</span>
          </Button>
        </div>
      {/if}
    {:else if kind === "missing-repository"}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <IconAlertTriangle class="size-5 text-warning" aria-hidden="true" />
          <h2>未找到目标仓库</h2>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          链接中的仓库未登记或已移除。请选择一个已登记仓库继续浏览，或登记新仓库。
        </p>
      </div>
      {#if repositories.length > 0}
        <div class="space-y-1">
          {#each repositories as repo (repo.id)}
            <ListButton
              variant="git-row"
              onclick={() => onSelectRepository(repo.id)}
            >
              <span class="min-w-0 flex-1 truncate">{repo.name}</span>
              <span class="truncate font-mono text-xs text-muted-foreground">
                {repo.anchorPath}
              </span>
            </ListButton>
          {/each}
        </div>
      {/if}
      {#if canAddRepository}
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onclick={onAddRepository}>
            <IconPlus aria-hidden="true" />
            <span>登记已有仓库…</span>
          </Button>
        </div>
      {/if}
    {:else if kind === "unavailable-repository"}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <IconAlertTriangle
            class="size-5 text-destructive"
            aria-hidden="true"
          />
          <h2>仓库当前不可用：{repository?.name ?? ""}</h2>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          {repository?.error ||
            "仓库路径缺失、权限不足或不再是有效的 Git 仓库，写操作与历史读取已禁用。"}
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onclick={onManageWorktrees}>
          <IconSettings aria-hidden="true" />
          <span>查看仓库详情…</span>
        </Button>
      </div>
    {:else if kind === "pick-worktree"}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <IconGitBranch class="size-5 text-info" aria-hidden="true" />
          <h2>请选择要写入的工作树</h2>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          {#if worktree?.kind === "bare"}
            当前目标为裸仓库（bare），不包含工作区文件，无法直接暂存或提交。请选择一个具体工作树或新建工作树。
          {:else}
            当前处于只读仓库视图，为避免误写其他分支目录，WebMux
            不会自动落到任意工作树。请显式选择一个工作树进入提交视图。
          {/if}
        </p>
      </div>
      {#if writableWorktrees.length > 0}
        {@render writableList(true)}
      {:else}
        <p class="text-xs text-muted-foreground">
          该仓库当前没有可写工作树，请先新建一个工作树。
        </p>
      {/if}
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="git-primary" onclick={onNewWorktree}>
          <IconPlus aria-hidden="true" />
          <span>新建工作树…</span>
        </Button>
        {#if onBackToLog}
          <Button size="sm" variant="outline" onclick={onBackToLog}>
            <IconHistory aria-hidden="true" />
            <span>返回历史 (Log)</span>
          </Button>
        {/if}
      </div>
    {:else}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <IconAlertTriangle class="size-5 text-warning" aria-hidden="true" />
          <h2>目标工作树不可用</h2>
        </div>
        <p class="text-xs leading-relaxed text-muted-foreground">
          {#if !worktree}
            工作树记录不存在或已被移除，已禁用暂存与提交写操作。仓库历史仍可只读浏览。
          {:else if worktree.locked}
            工作树（{worktree.path}）处于锁定状态，已禁用暂存与提交写操作。您可以管理工作树或切换到其他可用工作树。
          {:else}
            工作树目录（{worktree.path}）缺失或待清理（prunable），已禁用写操作。您可以修复/管理工作树或切换到其他可用工作树。
          {/if}
        </p>
      </div>
      {#if writableWorktrees.length > 0}
        {@render writableList(false)}
      {/if}
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onclick={onManageWorktrees}>
          <IconSettings aria-hidden="true" />
          <span>管理或修复工作树…</span>
        </Button>
        <Button
          size="sm"
          variant="outline"
          onclick={() => onSelectWorktree(undefined)}
        >
          <IconHistory aria-hidden="true" />
          <span>切换到只读仓库视图</span>
        </Button>
      </div>
    {/if}
  </div>
</div>
