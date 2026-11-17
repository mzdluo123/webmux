<script lang="ts">
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";

  type GitChangeHunk = GitWorkingChange["hunks"][number];

  let {
    changeId,
    hunks,
    hunkIndex,
    stageMode,
    busy,
    stale,
    canAddRework,
    onJumpToHunk,
    onAddHunkToRework,
    onStageHunk,
    onUnstageHunk,
    onDiscardHunk,
  }: {
    changeId: string;
    hunks: readonly GitChangeHunk[];
    hunkIndex: number;
    stageMode: "stage" | "unstage" | "none";
    busy: boolean;
    stale: boolean;
    canAddRework: boolean;
    onJumpToHunk: (index: number) => void;
    onAddHunkToRework?: (hunk: GitChangeHunk) => void;
    onStageHunk: (changeId: string, hunkId: string) => void;
    onUnstageHunk: (changeId: string, hunkId: string) => void;
    onDiscardHunk?: (changeId: string, hunkId: string) => void;
  } = $props();
</script>

<div
  class="shrink-0 border-b bg-background px-2.5 py-1.5"
  aria-label={m.s6_hunk_strip_aria()}
  data-testid="git-hunk-action-strip"
>
  <div
    class="mb-1 flex min-w-0 flex-wrap items-center justify-between gap-1 text-xs"
  >
    <span class="font-semibold text-muted-foreground">
      {m.s6_hunk_strip_heading()}
    </span>
    <span class="font-mono text-muted-foreground">
      {m.s6_hunk_strip_total({ count: String(hunks.length) })}
    </span>
  </div>
  <ScrollArea viewportClasses="max-h-28">
    <div class="space-y-1">
      {#each hunks as hunk, idx (hunk.id)}
        <div
          class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 rounded-sm border bg-background px-2 py-1"
          data-hunk-id={hunk.id}
        >
          <div class="min-w-0 basis-36 flex-1">
            <ListButton
              variant="compact-row"
              aria-selected={hunkIndex === idx}
              onclick={() => onJumpToHunk(idx)}
            >
              <span class="shrink-0 font-mono text-xs text-muted-foreground">
                #{idx + 1}
              </span>
              <span
                class="min-w-0 flex-1 truncate font-mono text-xs"
                title={hunk.header}
              >
                {hunk.header}
              </span>
            </ListButton>
          </div>
          <div class="flex min-w-0 flex-wrap items-center gap-1">
            {#if canAddRework && onAddHunkToRework}
              <Button
                size="sm"
                variant="ghost"
                onclick={() => onAddHunkToRework(hunk)}
                data-testid="git-hunk-add-rework"
              >
                <IconMessagePlus aria-hidden="true" />
                {m.s6_hunk_add_comment()}
              </Button>
            {/if}
            {#if stageMode === "stage"}
              {#if onDiscardHunk}
                <Button
                  size="sm"
                  variant="destructive-outline"
                  disabled={busy || stale}
                  onclick={() => onDiscardHunk(changeId, hunk.id)}
                  data-testid="git-hunk-discard-btn"
                >
                  <IconTrash aria-hidden="true" />
                  {m.s6_hunk_discard_btn()}
                </Button>
              {/if}
              <Button
                size="sm"
                variant="outline"
                disabled={busy || stale}
                onclick={() => onStageHunk(changeId, hunk.id)}
              >
                <IconPlus aria-hidden="true" />
                {m.s6_hunk_stage_btn()}
              </Button>
            {:else}
              <Button
                size="sm"
                variant="outline"
                disabled={busy || stale}
                onclick={() => onUnstageHunk(changeId, hunk.id)}
              >
                <IconMinus aria-hidden="true" />
                {m.s6_hunk_unstage_btn()}
              </Button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </ScrollArea>
</div>
