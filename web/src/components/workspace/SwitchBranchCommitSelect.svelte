<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Label } from "$lib/components/ui/label";
  import * as Select from "$lib/components/ui/select";
  import {
    commitChoiceLabel,
    type CommitChoice,
  } from "../../domains/workspace/commit-choices";

  let {
    commit = $bindable(""),
    commitRows,
    commitsRead = false,
    commitsError = "",
    busy = false,
  }: {
    commit: string;
    commitRows: readonly CommitChoice[];
    commitsRead?: boolean;
    commitsError?: string;
    busy?: boolean;
  } = $props();

  const chosenCommit = $derived(commitRows.find((row) => row.value === commit));
</script>

<div class="space-y-2">
  <Label for="switch-commit">{m.s8_label_commit()}</Label>
  <Select.Root
    type="single"
    bind:value={commit}
    disabled={busy || !commitRows.length}
  >
    <Select.Trigger id="switch-commit" class="w-full">
      {#if chosenCommit}
        <span class="min-w-0 flex-1 truncate" title={chosenCommit.subject}
          >{chosenCommit.subject} · {chosenCommit.shortHash}</span
        >
      {:else}
        <span class="text-muted-foreground">
          {commitsRead ? m.s8_select_commit() : m.s8_reading_recent_commits()}
        </span>
      {/if}
    </Select.Trigger>
    <Select.Content
      class="max-h-[min(24rem,var(--bits-select-content-available-height))]"
    >
      {#each commitRows as row (row.value)}
        <Select.Item
          value={row.value}
          label={row.subject}
          aria-label={commitChoiceLabel(row)}
          >{commitChoiceLabel(row)}</Select.Item
        >
      {/each}
    </Select.Content>
  </Select.Root>
  {#if commitsError}<p role="alert" class="text-xs text-destructive">
      {m.s8_err_load_recent_commits({ error: commitsError })}
    </p>{:else if commitsRead && !commitRows.length}<p
      class="text-xs text-muted-foreground"
    >
      {m.s8_worktree_has_no_commits()}
    </p>{/if}
  <p class="text-xs text-muted-foreground">
    {m.s8_detach_commit_help()}
  </p>
</div>
