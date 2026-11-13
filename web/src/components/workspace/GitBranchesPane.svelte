<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconStar from "~icons/tabler/star";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitRowActionMenu from "./GitRowActionMenu.svelte";

  let {
    context,
    branches,
    onFilterInLog,
    onRequestCheckout,
    onOpenOccupiedWorktree,
    onOpenPush,
  }: {
    context: GitWorktreeContext;
    branches: readonly GitBranchItem[];
    onFilterInLog: (branch: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
    onOpenPush: (branch: string) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  let query = $state("");
  let selectedName = $state<string | null>(null);

  const filtered = $derived(
    branches.filter((b) =>
      query.trim()
        ? b.name.toLowerCase().includes(query.trim().toLowerCase()) ||
          b.subject.toLowerCase().includes(query.trim().toLowerCase())
        : true,
    ),
  );

  const currentBranch = $derived(
    filtered.filter(
      (b) => b.kind === "local" && b.name === context.checkedOutBranch,
    ),
  );
  const favorites = $derived(
    filtered.filter((b) => b.favorite && b.name !== context.checkedOutBranch),
  );
  const localBranches = $derived(
    filtered.filter(
      (b) =>
        b.kind === "local" &&
        !b.favorite &&
        b.name !== context.checkedOutBranch,
    ),
  );
  const remoteBranches = $derived(filtered.filter((b) => b.kind === "remote"));

  const activeBranch = $derived(
    branches.find((b) => b.name === selectedName) ??
      currentBranch[0] ??
      filtered[0] ??
      null,
  );
</script>

{#snippet branchRow(item: GitBranchItem)}
  {@const isCurrent =
    item.kind === "local" && item.name === context.checkedOutBranch}
  <div class="group/row relative">
    <ListButton
      variant="git-row"
      data-has-action="true"
      aria-selected={activeBranch?.name === item.name}
      onclick={() => (selectedName = item.name)}
    >
      <IconGitBranch
        class="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      {#if item.favorite}
        <IconStar class="size-3 shrink-0 text-warning" aria-hidden="true" />
      {/if}
      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        <span
          class="min-w-[6ch] max-w-full flex-1 truncate font-medium"
          title={item.name}
        >
          {item.name}
        </span>

        {#if isCurrent}
          <Badge variant="info" class="max-w-full">
            <span class="truncate">当前工作树</span>
          </Badge>
        {:else if item.occupiedBy}
          <Badge variant="warning" class="max-w-full">
            <span class="truncate">其他工作树已检出</span>
          </Badge>
        {/if}

        {#if item.upstreamGone}
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">上游已移除</span>
          </Badge>
        {:else if (item.ahead ?? 0) > 0 || (item.behind ?? 0) > 0}
          <Badge variant="outline">
            ↑{item.ahead ?? 0} ↓{item.behind ?? 0}
          </Badge>
        {/if}
      </div>
    </ListButton>

    <GitRowActionMenu label={`分支操作 ${item.name}`}>
      <DropdownMenu.Item onclick={() => onFilterInLog(item.name)}>
        在历史 (Log) 中筛选此分支
      </DropdownMenu.Item>
      {#if item.occupiedBy}
        <DropdownMenu.Item
          onclick={() =>
            item.occupiedBy &&
            onOpenOccupiedWorktree(
              item.occupiedBy.worktreeId,
              item.occupiedBy.worktreePath,
            )}
        >
          打开所在工作树
        </DropdownMenu.Item>
      {:else if item.kind === "local" && !isCurrent && context.worktreePath}
        <DropdownMenu.Item onclick={() => onRequestCheckout(item.name)}>
          检出到当前工作树…
        </DropdownMenu.Item>
      {/if}
      {#if item.kind === "local" && isCurrent}
        <DropdownMenu.Item onclick={() => onOpenPush(item.name)}>
          推送预览…
        </DropdownMenu.Item>
      {/if}
    </GitRowActionMenu>
  </div>
{/snippet}

{#snippet branchGroup(title: string, items: readonly GitBranchItem[])}
  {#if items.length > 0}
    <div class="space-y-1">
      <div class="flex items-center justify-between px-1.5">
        <span class="text-xs font-semibold text-muted-foreground">{title}</span>
        <Badge variant="outline">{items.length}</Badge>
      </div>
      <div class="space-y-0.5">
        {#each items as item (`${item.kind}:${item.name}`)}
          {@render branchRow(item)}
        {/each}
      </div>
    </div>
  {/if}
{/snippet}

<div
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface lg:flex-row"
  data-slot="git-branches-pane"
>
  <section
    class="flex min-h-0 min-w-0 flex-1 flex-col border-b border-git-border lg:border-r lg:border-b-0"
    aria-label="分支与上游列表"
  >
    <div class="border-b border-git-border bg-git-panel p-2.5">
      <Input
        size="sm"
        value={query}
        placeholder="搜索本地或远程分支…"
        aria-label="搜索分支"
        oninput={(e) => (query = e.currentTarget.value)}
      />
    </div>

    <div class="min-h-0 flex-1 space-y-3 overflow-y-auto p-2">
      {@render branchGroup("当前检出 (Current)", currentBranch)}
      {@render branchGroup("常用分支 (Favorites)", favorites)}
      {@render branchGroup("本地分支 (Local)", localBranches)}
      {@render branchGroup("远程跟踪分支 (Remote)", remoteBranches)}
      {#if filtered.length === 0}
        <p class="p-3 text-xs text-muted-foreground">没有匹配的分支。</p>
      {/if}
    </div>
  </section>

  <section
    class={desktop.current
      ? "flex w-96 shrink-0 flex-col bg-git-panel p-4"
      : "flex shrink-0 flex-col bg-git-panel p-3"}
    aria-label="选中分支详情与工作树占用"
  >
    {#if !activeBranch}
      <p class="text-xs text-muted-foreground">
        选择任意分支查看上游与工作树状态。
      </p>
    {:else}
      <div class="space-y-3">
        <div>
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <h3
              class="font-mono text-sm font-semibold break-all text-foreground"
            >
              {activeBranch.name}
            </h3>
            <Badge variant="outline">
              {activeBranch.kind === "local" ? "本地分支" : "远程引用"}
            </Badge>
          </div>
          <p class="mt-1 text-xs wrap-anywhere text-muted-foreground">
            最新提交：{activeBranch.shortHead} · {activeBranch.subject}
          </p>
        </div>

        {#if activeBranch.upstream}
          <div
            class="rounded-md border border-git-border bg-git-surface p-2.5 text-xs"
          >
            <p class="wrap-anywhere">
              上游跟踪：<span class="font-mono">{activeBranch.upstream}</span>
              {#if activeBranch.upstreamGone}
                <span class="text-destructive">（远程分支已不存在）</span>
              {:else}
                <span
                  >（领先 {activeBranch.ahead ?? 0} · 落后 {activeBranch.behind ??
                    0}）</span
                >
              {/if}
            </p>
          </div>
        {:else if activeBranch.kind === "local"}
          <p class="text-xs text-muted-foreground">
            尚未配置远程上游分支，推送时将执行首次发布（Publish Branch）。
          </p>
        {/if}

        {#if activeBranch.occupiedBy}
          <InlineNotice tone="warning">
            该分支已在另一工作树 <span class="font-mono break-all"
              >{activeBranch.occupiedBy.worktreePath}</span
            >
            检出（当前活跃会话：{activeBranch.occupiedBy.activeSessions} 个）。Git
            不允许两个工作树同时检出同一本地分支，请直接前往该工作树继续操作。
          </InlineNotice>
        {/if}

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <Button
            size="sm"
            variant="outline"
            onclick={() => onFilterInLog(activeBranch.name)}
          >
            在历史中查看提交
          </Button>

          {#if activeBranch.occupiedBy}
            <Button
              size="sm"
              variant="git-primary"
              onclick={() =>
                activeBranch.occupiedBy &&
                onOpenOccupiedWorktree(
                  activeBranch.occupiedBy.worktreeId,
                  activeBranch.occupiedBy.worktreePath,
                )}
            >
              打开所在工作树
            </Button>
          {:else if activeBranch.kind === "local" && activeBranch.name === context.checkedOutBranch}
            <Button
              size="sm"
              variant="git-primary"
              onclick={() => onOpenPush(activeBranch.name)}
            >
              推送预览…
            </Button>
          {:else if activeBranch.kind === "local" && context.worktreePath}
            <Button
              size="sm"
              variant="git-primary"
              onclick={() => onRequestCheckout(activeBranch.name)}
            >
              检出分支…
            </Button>
          {/if}
        </div>
      </div>
    {/if}
  </section>
</div>
