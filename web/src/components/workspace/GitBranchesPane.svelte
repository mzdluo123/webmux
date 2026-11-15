<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { Remote } from "../../contracts/schemas";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    groupAndSortGitBranches,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import type { GitRemotesController } from "../../hooks/workspace/git-remotes-controller.svelte";
  import GitBranchDetailCard from "./GitBranchDetailCard.svelte";
  import GitBranchListItem from "./GitBranchListItem.svelte";
  import GitRemotesSection from "./GitRemotesSection.svelte";

  let {
    context,
    branches,
    remotes,
    remotesController,
    branchFilter = null,
    onFilterInLog,
    onRequestCheckout,
    onOpenOccupiedWorktree,
    onOpenPush,
    onBranchAction,
    onOpenCreateTagFromHead,
  }: {
    context: GitWorktreeContext;
    branches: readonly GitBranchItem[];
    remotes?: readonly Remote[];
    remotesController?: GitRemotesController;
    branchFilter?: string | null;
    onFilterInLog: (branch: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
    onOpenPush: (branch: string) => void;
    onBranchAction?: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onOpenCreateTagFromHead?: () => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  let query = $state("");
  let selectedKey = $state<string | null>(null);
  let listEl = $state<HTMLElement | null>(null);

  const grouped = $derived(
    groupAndSortGitBranches(branches, context.checkedOutBranch, query),
  );
  const allVisible = $derived([
    ...grouped.current,
    ...grouped.favorites,
    ...grouped.local.filter((b) => !b.favorite),
    ...grouped.remote.filter((b) => !b.favorite),
    ...grouped.tags,
  ]);
  const activeBranch = $derived(
    branches.find((b) => `${b.kind}:${b.name}` === selectedKey) ??
      grouped.current[0] ??
      allVisible[0] ??
      null,
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
            rowRole="option"
            selected={activeBranch?.kind === item.kind &&
              activeBranch?.name === item.name}
            {branchFilter}
            {sectionId}
            currentLabel={m.s6_current_worktree_badge()}
            occupiedLabelStyle="verbose"
            compareBaseOid={context.headOid ?? null}
            compareTargetLabel={m.s6_compare_with_head()}
            menuLabel={m.s6_branch_actions_menu_label({ name: item.name })}
            onSelectRow={() => (selectedKey = `${item.kind}:${item.name}`)}
            onAction={routeBranchOp}
          />
        {/each}
      </div>
    </div>
  {/if}
{/snippet}

<div class="h-full min-h-0 min-w-0 bg-background" data-slot="git-branches-pane">
  <ScrollArea class="h-full min-h-8">
    <div
      class={desktop.current
        ? "flex h-full min-h-0 min-w-0 flex-row"
        : "flex min-h-full min-w-0 flex-col"}
    >
      <section
        class={desktop.current
          ? "flex min-h-0 min-w-0 flex-1 flex-col border-r"
          : "flex shrink-0 min-w-0 flex-col border-b"}
        aria-label={m.s6_branches_list_section_aria()}
      >
        <div
          class="flex min-w-0 flex-wrap items-center gap-2 border-b bg-background p-2.5"
        >
          <div class="min-w-0 flex-1 basis-36">
            <Input
              size="sm"
              value={query}
              placeholder={m.s6_branches_search_placeholder()}
              aria-label={m.s6_branches_search_aria()}
              oninput={(e) => (query = e.currentTarget.value)}
            />
          </div>
          {#if onOpenCreateTagFromHead && context.headOid}
            <Button
              size="sm"
              variant="outline"
              onclick={onOpenCreateTagFromHead}
              data-testid="git-branches-create-tag-head-btn"
            >
              <IconTag aria-hidden="true" />
              {m.s6_create_tag_ellipsis()}
            </Button>
          {/if}
        </div>

        <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child options. -->
        <ScrollArea
          bind:viewportRef={listEl}
          class="min-h-8 flex-1"
          viewportProps={{
            role: "listbox",
            "aria-label": m.s6_branches_listbox_aria(),
            "aria-orientation": "vertical",
            onkeydown: handleBranchListKeydown,
          }}
        >
          <div class="space-y-3 p-2">
            {@render branchGroup(
              "current",
              m.s6_branches_group_current(),
              grouped.current,
            )}
            {@render branchGroup(
              "favorites",
              m.s6_branches_group_favorites(),
              grouped.favorites,
            )}
            {@render branchGroup(
              "local",
              m.s6_branches_group_local(),
              grouped.local.filter((b) => !b.favorite),
            )}
            {@render branchGroup(
              "remote",
              m.s6_branches_group_remote(),
              grouped.remote.filter((b) => !b.favorite),
            )}
            {@render branchGroup(
              "tags",
              m.s6_branches_group_tags(),
              grouped.tags,
            )}

            {#if allVisible.length === 0}
              <p class="p-3 text-xs text-muted-foreground">
                {m.s6_branches_no_matches()}
              </p>
            {/if}
          </div>
        </ScrollArea>
      </section>

      <section
        class={desktop.current
          ? "flex w-96 min-h-0 shrink-0 flex-col bg-background"
          : "flex shrink-0 flex-col bg-background"}
        aria-label={m.s6_branches_detail_section_aria()}
      >
        <ScrollArea class="min-h-8 flex-1">
          <div class={desktop.current ? "space-y-4 p-4" : "space-y-4 p-3"}>
            <GitBranchDetailCard
              {activeBranch}
              {context}
              {branchFilter}
              onRouteOp={routeBranchOp}
            />

            {#if remotes !== undefined || remotesController}
              <div class="border-t pt-3">
                <GitRemotesSection
                  remotes={remotes ?? []}
                  controller={remotesController}
                  compact
                />
              </div>
            {/if}
          </div>
        </ScrollArea>
      </section>
    </div>
  </ScrollArea>
</div>
