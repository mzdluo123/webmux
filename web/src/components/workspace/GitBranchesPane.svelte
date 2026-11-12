<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    groupAndSortGitBranches,
    resolveGitBranchActions,
    resolveGitBranchUpstream,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitBranchListItem from "./GitBranchListItem.svelte";

  let {
    context,
    branches,
    branchFilter = null,
    onFilterInLog,
    onRequestCheckout,
    onOpenOccupiedWorktree,
    onOpenPush,
    onBranchAction,
  }: {
    context: GitWorktreeContext;
    branches: readonly GitBranchItem[];
    branchFilter?: string | null;
    onFilterInLog: (branch: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
    onOpenPush: (branch: string) => void;
    onBranchAction?: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  let query = $state("");
  let selectedKey = $state<string | null>(null);
  let listEl = $state<HTMLDivElement>();

  const grouped = $derived(
    groupAndSortGitBranches(branches, context.checkedOutBranch, query),
  );
  const allVisible = $derived([
    ...grouped.current,
    ...grouped.favorites,
    ...grouped.local.filter((b) => !b.favorite),
    ...grouped.remote.filter((b) => !b.favorite),
  ]);
  const activeBranch = $derived(
    branches.find((b) => `${b.kind}:${b.name}` === selectedKey) ??
      grouped.current[0] ??
      allVisible[0] ??
      null,
  );
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

  function routeBranchOp(kind: GitSharedBranchActionKind, item: GitBranchItem) {
    if (kind === "filter-log") return onFilterInLog(item.name);
    if (onBranchAction) return onBranchAction(kind, item);
    if (kind === "checkout") return onRequestCheckout(item.name);
    if (kind === "push") return onOpenPush(item.name);
    if (kind === "open-occupied-worktree" && item.occupiedBy) {
      onOpenOccupiedWorktree(
        item.occupiedBy.worktreeId,
        item.occupiedBy.worktreePath,
      );
    }
  }

  function focusBranchButton(index: number) {
    const btns = listEl?.querySelectorAll<HTMLButtonElement>(
      "[data-branch-item-btn]",
    );
    btns?.[index]?.focus();
  }

  function handleBranchListKeydown(e: KeyboardEvent) {
    if (allVisible.length === 0) return;
    if (!(e.target instanceof HTMLElement)) return;
    if (!e.target.hasAttribute("data-branch-item-btn")) return;

    const currentIdx = Math.max(
      0,
      allVisible.findIndex(
        (b) => b.kind === activeBranch?.kind && b.name === activeBranch?.name,
      ),
    );
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(allVisible.length - 1, currentIdx + 1);
      const next = allVisible[nextIdx]!;
      selectedKey = `${next.kind}:${next.name}`;
      focusBranchButton(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      const prev = allVisible[prevIdx]!;
      selectedKey = `${prev.kind}:${prev.name}`;
      focusBranchButton(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      const first = allVisible[0]!;
      selectedKey = `${first.kind}:${first.name}`;
      focusBranchButton(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = allVisible.length - 1;
      const last = allVisible[lastIdx]!;
      selectedKey = `${last.kind}:${last.name}`;
      focusBranchButton(lastIdx);
    }
  }
</script>

{#snippet branchGroup(
  sectionId: string,
  title: string,
  items: readonly GitBranchItem[],
)}
  {#if items.length > 0}
    <div class="space-y-1">
      <div class="flex items-center justify-between px-1.5">
        <span class="text-xs font-semibold text-muted-foreground">{title}</span>
        <Badge variant="outline">{items.length}</Badge>
      </div>
      <div class="space-y-0.5">
        {#each items as item (`${sectionId}:${item.kind}:${item.name}`)}
          <GitBranchListItem
            branch={item}
            {context}
            selected={activeBranch?.kind === item.kind &&
              activeBranch?.name === item.name}
            {branchFilter}
            {sectionId}
            currentLabel="当前工作树"
            occupiedLabelStyle="verbose"
            compareBaseOid={context.headOid ?? null}
            compareTargetLabel="与当前 HEAD 比较"
            menuLabel={`分支操作 ${item.name}`}
            onSelectRow={() => (selectedKey = `${item.kind}:${item.name}`)}
            onAction={routeBranchOp}
          />
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

    <!-- Interactive container delegates ArrowUp/ArrowDown/Home/End across child branch buttons. -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      bind:this={listEl}
      class="min-h-0 flex-1 space-y-3 overflow-y-auto p-2"
      role="region"
      aria-label="分支列表"
      onkeydown={handleBranchListKeydown}
    >
      {@render branchGroup("current", "当前检出 (Current)", grouped.current)}
      {@render branchGroup(
        "favorites",
        "收藏分支 (Favorites)",
        grouped.favorites,
      )}
      {@render branchGroup(
        "local",
        "本地分支 (Local)",
        grouped.local.filter((b) => !b.favorite),
      )}
      {@render branchGroup(
        "remote",
        "远程跟踪分支 (Remote)",
        grouped.remote.filter((b) => !b.favorite),
      )}
      {#if allVisible.length === 0}
        <p class="p-3 text-xs text-muted-foreground">没有匹配的分支。</p>
      {/if}
    </div>
  </section>

  <section
    class={desktop.current
      ? "flex w-96 shrink-0 flex-col overflow-y-auto bg-git-panel p-4"
      : "flex shrink-0 flex-col bg-git-panel p-3"}
    aria-label="选中分支详情与工作树占用"
  >
    {#if !activeBranch}
      <p class="text-xs text-muted-foreground">
        选择任意分支查看上游、工作树占用与可执行操作。
      </p>
    {:else}
      {@const isCurrent =
        activeBranch.kind === "local" &&
        activeBranch.name === context.checkedOutBranch}
      {@const pushAct = activeActions.find((a) => a.kind === "push")}
      {@const ffAct = activeActions.find((a) => a.kind === "fast-forward")}
      {@const delAct = activeActions.find((a) => a.kind === "delete")}
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

        {#if activeBranch.kind === "local" && activeUpstream}
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
            检出（当前活跃会话：{activeBranch.occupiedBy.activeSessions} 个）。Git
            不允许两个工作树同时检出同一本地分支，请直接前往该工作树或从该提交新建工作树分支。
          </InlineNotice>
        {/if}

        <div class="flex flex-wrap items-center gap-2 pt-1">
          <Button
            size="sm"
            variant="outline"
            onclick={() => routeBranchOp("filter-log", activeBranch)}
          >
            在历史中查看提交
          </Button>

          {#if activeBranch.occupiedBy}
            <Button
              size="sm"
              variant="git-primary"
              onclick={() =>
                routeBranchOp("open-occupied-worktree", activeBranch)}
            >
              打开所在工作树
            </Button>
            <Button
              size="sm"
              variant="outline"
              onclick={() => routeBranchOp("open-new-worktree", activeBranch)}
            >
              从该提交新建分支工作树…
            </Button>
          {:else if isCurrent && pushAct}
            <Button
              size="sm"
              variant="git-primary"
              disabled={pushAct.disabled}
              title={pushAct.disabledReason}
              onclick={() => routeBranchOp("push", activeBranch)}
            >
              {pushAct.label}
            </Button>
          {:else if activeBranch.kind !== "tag" && context.worktreePath}
            <Button
              size="sm"
              variant="git-primary"
              onclick={() => routeBranchOp("checkout", activeBranch)}
            >
              {activeBranch.kind === "remote"
                ? "检出并跟踪…"
                : "检出到当前工作树…"}
            </Button>
          {/if}

          {#if !activeBranch.occupiedBy && activeBranch.kind !== "tag"}
            <Button
              size="sm"
              variant="outline"
              onclick={() => routeBranchOp("create-branch-from", activeBranch)}
            >
              从此分支新建分支…
            </Button>
            <Button
              size="sm"
              variant="outline"
              onclick={() => routeBranchOp("open-new-worktree", activeBranch)}
            >
              在新工作树打开…
            </Button>
          {/if}

          {#if activeBranch.kind === "local"}
            {#if ffAct}
              <Button
                size="sm"
                variant="outline"
                disabled={ffAct.disabled}
                title={ffAct.disabledReason}
                onclick={() => routeBranchOp("fast-forward", activeBranch)}
              >
                {ffAct.label}
              </Button>
            {/if}
            {#if delAct}
              <Button
                size="sm"
                variant="destructive"
                disabled={delAct.disabled}
                title={delAct.disabledReason}
                onclick={() => routeBranchOp("delete", activeBranch)}
              >
                {delAct.label}
              </Button>
            {/if}
          {/if}
        </div>
      </div>
    {/if}
  </section>
</div>
