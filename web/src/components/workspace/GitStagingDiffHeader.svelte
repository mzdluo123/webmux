<script lang="ts">
  import GitFileStatusIcon from "../shared/GitFileStatusIcon.svelte";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconListCheck from "~icons/tabler/list-check";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import { formatStagingComparisonEndpoints } from "../../hooks/workspace/git-staging-diff.svelte";
  import GitDiffStyleSwitch from "./GitDiffStyleSwitch.svelte";

  let {
    change,
    hunkIndex,
    phase,
    diffMode,
    stageMode,
    canStageLines,
    linePickerOpen,
    canDiscardCurrentFile,
    busy,
    stale,
    onJumpToHunk,
    onToggleLinePicker,
    onDiffModeChange,
    onDiscardFile,
    onStageChange,
    onUnstageChange,
  }: {
    change: GitWorkingChange;
    hunkIndex: number;
    phase: string;
    diffMode: "unified" | "split";
    stageMode: "stage" | "unstage" | "none";
    canStageLines: boolean;
    linePickerOpen: boolean;
    canDiscardCurrentFile: boolean;
    busy: boolean;
    stale: boolean;
    onJumpToHunk: (index: number) => void;
    onToggleLinePicker: () => void;
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onDiscardFile?: (changeId: string, path: string) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
  } = $props();

  const endpoints = $derived(formatStagingComparisonEndpoints(change));
  const pathTitle = $derived(
    change.oldPath && change.oldPath !== change.path
      ? `${change.oldPath} → ${change.path}`
      : change.path,
  );
</script>

<header
  class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5"
>
  <div class="flex min-w-0 basis-48 flex-1 flex-wrap items-center gap-1.5">
    <GitFileStatusIcon status={change.status} group={change.group} />
    <Badge variant="outline" class="max-w-full">
      <span class="truncate font-mono"
        >{endpoints.left} → {endpoints.right}</span
      >
    </Badge>
    <span
      class="min-w-0 truncate font-mono text-xs font-medium text-foreground"
      title={pathTitle}
    >
      {pathTitle}
    </span>
  </div>

  <div class="flex min-w-0 flex-wrap items-center gap-1.5">
    {#if change.hunks.length > 0}
      <div class="flex items-center gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          disabled={hunkIndex <= 0}
          aria-label={m.s6_staging_prev_hunk()}
          title={m.s6_staging_prev_hunk()}
          onclick={() => onJumpToHunk(hunkIndex - 1)}
        >
          <IconArrowUp aria-hidden="true" />
        </Button>
        <span class="font-mono text-xs text-muted-foreground">
          {Math.min(hunkIndex + 1, change.hunks.length)}/{change.hunks.length}
        </span>
        <Button
          size="icon-sm"
          variant="ghost"
          disabled={hunkIndex >= change.hunks.length - 1}
          aria-label={m.s6_staging_next_hunk()}
          title={m.s6_staging_next_hunk()}
          onclick={() => onJumpToHunk(hunkIndex + 1)}
        >
          <IconArrowDown aria-hidden="true" />
        </Button>
      </div>
    {/if}

    {#if canStageLines}
      <Button
        size="sm"
        variant="outline"
        aria-pressed={linePickerOpen}
        onclick={onToggleLinePicker}
        data-testid="git-toggle-line-selector"
      >
        <IconListCheck aria-hidden="true" />
        {m.s6_staging_line_picker_btn()}
      </Button>
    {/if}

    {#if phase === "ready" && !(linePickerOpen && canStageLines)}
      <GitDiffStyleSwitch value={diffMode} onChange={onDiffModeChange} />
    {/if}

    {#if canDiscardCurrentFile && onDiscardFile}
      <Button
        size="sm"
        variant="destructive-outline"
        disabled={busy || stale}
        onclick={() => onDiscardFile(change.id, change.path)}
        data-testid="git-diff-discard-file-btn"
      >
        <IconTrash aria-hidden="true" />
        {m.s6_staging_discard_file_btn()}
      </Button>
    {/if}

    {#if stageMode === "stage"}
      <Button
        size="sm"
        variant="outline"
        disabled={busy || stale}
        onclick={() => onStageChange(change.id)}
      >
        <IconPlus aria-hidden="true" />
        {m.s6_staging_stage_file_btn()}
      </Button>
    {:else if stageMode === "unstage"}
      <Button
        size="sm"
        variant="outline"
        disabled={busy || stale}
        onclick={() => onUnstageChange(change.id)}
      >
        <IconMinus aria-hidden="true" />
        {m.s6_staging_unstage_file_btn()}
      </Button>
    {/if}
  </div>
</header>
