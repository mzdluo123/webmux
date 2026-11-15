<script lang="ts">
  import type { Snippet } from "svelte";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import type {
    GitSharedBranchActionItem,
    GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import GitRowActionMenu from "./GitRowActionMenu.svelte";

  let {
    branch,
    actions,
    label,
    sectionId,
    onAction,
    children,
  }: {
    branch: GitBranchItem;
    actions: readonly GitSharedBranchActionItem[];
    label: string;
    sectionId?: string;
    onAction: (kind: GitSharedBranchActionKind, branch: GitBranchItem) => void;
    children: Snippet;
  } = $props();

  let open = $state(false);

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    open = true;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
      e.preventDefault();
      open = true;
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="group/row relative"
  data-ref-section={sectionId}
  data-branch-row={branch.name}
  oncontextmenu={handleContextMenu}
  onkeydown={handleKeydown}
>
  {@render children()}

  <GitRowActionMenu {label} bind:open>
    {#each actions as act (act.kind)}
      {#if act.separatorBefore}
        <DropdownMenu.Separator />
      {/if}
      <DropdownMenu.Item
        variant={act.destructive ? "destructive" : "default"}
        disabled={act.disabled}
        title={act.disabledReason}
        onSelect={() => {
          if (!act.disabled) onAction(act.kind, branch);
        }}
      >
        <div class="flex min-w-0 flex-col">
          <span>{act.label}</span>
          {#if act.disabled && act.disabledReason}
            <span class="text-xs text-muted-foreground">
              {act.disabledReason}
            </span>
          {/if}
        </div>
      </DropdownMenu.Item>
    {/each}
  </GitRowActionMenu>
</div>
