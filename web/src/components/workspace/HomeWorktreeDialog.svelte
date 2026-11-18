<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import { placeArchived } from "../../domains/sessions/archived";
  import {
    homeLayout,
    worktreeTitle,
  } from "../../domains/workspace/home-worktrees";
  import type {
    ArchivedSession,
    HomeWorktrees,
    Repository,
    SessionInfo,
    WorktreePins,
  } from "../../contracts/schemas";
  import { isWorktreePinned } from "../../domains/workspace/worktree-pins";
  import { worktreePins } from "../../domains/workspace/worktree-pins.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

  // Which of a repository's worktrees the home page lists. Every worktree is
  // one checkable row; a worktree with sessions stays listed whatever its
  // check, so the row says so instead of hiding it.
  let {
    repository,
    sessions,
    records,
    home,
    pins = worktreePins.value,
    ontoggle,
    onmanage,
    onclose,
  }: {
    repository: Repository;
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    home: HomeWorktrees;
    pins?: WorktreePins;
    ontoggle: (worktreeId: string, pinned: boolean) => void;
    onmanage?: () => void;
    onclose: () => void;
  } = $props();
  let search = $state("");
  const layout = $derived(
    homeLayout(
      home,
      repository,
      sessions,
      placeArchived(
        records.filter((record) => !sessions.some((s) => s.id === record.id)),
        [repository],
      ).repositories.get(repository.id)?.worktrees,
      pins,
    ),
  );
  // Every worktree in a stable order, so a row does not jump when its check
  // changes; a worktree with sessions is listed even when not pinned.
  const worktrees = $derived(layout.trees);
  const pinned = $derived(
    new Set(layout.shown.filter((w) => w.pinned).map((w) => w.tree.id)),
  );
  const active = $derived(
    new Set(layout.shown.filter((w) => !w.pinned).map((w) => w.tree.id)),
  );
</script>

<ResponsiveDialog
  title={m.s8_home_wt_title({ name: repository.name })}
  description={m.s8_home_wt_desc()}
  {onclose}
>
  <Command.Root
    label={m.s8_home_wt_command_label()}
    class="h-auto min-h-0 min-w-0 flex-1"
    filter={(value, term, keywords) =>
      [value, ...(keywords ?? [])].some((text) =>
        text.toLowerCase().includes(term.trim().toLowerCase()),
      )
        ? 1
        : 0}
    disableInitialScroll
  >
    <Command.Input
      bind:value={search}
      placeholder={m.s8_home_wt_search_placeholder()}
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck={false}
    />
    <Command.List
      aria-label={m.s8_home_wt_list_label()}
      aria-multiselectable="true"
      class="max-h-72 min-h-0 flex-1"
    >
      <Command.Empty>{m.s8_home_wt_empty()}</Command.Empty>
      {#each worktrees as tree (tree.id)}
        <Command.Item
          value={tree.id}
          keywords={[worktreeTitle(tree), tree.path]}
          data-checked={pinned.has(tree.id)}
          aria-label={worktreeTitle(tree)}
          onSelect={() => ontoggle(tree.id, !pinned.has(tree.id))}
        >
          <span class="min-w-0 flex-1 space-y-0.5">
            <span class="flex flex-wrap items-center gap-1.5">
              <span class="font-medium break-all">{worktreeTitle(tree)}</span>
              {#if tree.kind === "main"}<Badge variant="secondary"
                  >{m.s8_badge_main_worktree()}</Badge
                >{/if}
              {#if isWorktreePinned(pins, repository.id, tree.id)}<Badge
                  variant="secondary">{m.s8_badge_pinned_worktree()}</Badge
                >{/if}
              {#if active.has(tree.id) && !pinned.has(tree.id)}<Badge
                  variant="outline">{m.s8_badge_active_always_shown()}</Badge
                >{/if}
              {#if !tree.available}<Badge variant="destructive"
                  >{m.s8_badge_unavailable()}</Badge
                >{/if}
            </span>
            <span
              class="block font-mono text-xs break-all text-muted-foreground"
              >{tree.path}</span
            >
          </span>
        </Command.Item>
      {/each}
    </Command.List>
  </Command.Root>
  <div
    class="flex flex-wrap items-center justify-between gap-2 border-t px-3 py-2 text-xs text-muted-foreground"
  >
    <span>{m.s8_home_wt_uncheck_hint()}</span>
    {#if onmanage}
      <Button
        size="xs"
        variant="ghost"
        onclick={() => {
          onclose();
          onmanage();
        }}>{m.s8_home_wt_batch_manage()}</Button
      >
    {/if}
  </div>
</ResponsiveDialog>
