<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import {
    commitChoiceLabel,
    type CommitChoice,
  } from "../../domains/workspace/commit-choices";
  import { Badge } from "$lib/components/ui/badge";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Select from "$lib/components/ui/select";
  import AdvancedSection from "../shared/AdvancedSection.svelte";

  let {
    mode,
    deferred = false,
    path = $bindable(""),
    base = $bindable(""),
    disabled = false,
    loading = false,
    hasData = false,
    commitRows,
    selectedCommit,
    commitsCount,
    commitsLoading = false,
    commitsError = "",
    onpathedited,
    onbaseedited,
  }: {
    mode: "new" | "existing";
    deferred?: boolean;
    path: string;
    base: string;
    disabled?: boolean;
    loading?: boolean;
    hasData?: boolean;
    commitRows: readonly CommitChoice[];
    selectedCommit: CommitChoice | undefined;
    commitsCount: number;
    commitsLoading?: boolean;
    commitsError?: string;
    onpathedited: () => void;
    onbaseedited: () => void;
  } = $props();
</script>

{#snippet commitDetails(item: CommitChoice)}
  <span class="grid min-w-0 flex-1 gap-0.5">
    <span class="truncate text-sm font-medium" title={item.subject}
      >{item.subject}</span
    >
    <span
      class="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground"
    >
      <span class="font-mono">{item.shortHash}</span>
      {#each item.tags as tag (tag)}
        <Badge variant={tag === m.s8_tip_remote_latest() ? "info" : "secondary"}
          >{tag}</Badge
        >
      {/each}
      {#if item.age}<span>{item.age}</span>{/if}
    </span>
  </span>
{/snippet}

{#if mode === "new" && hasData && !commitRows.length && !commitsLoading}<p
    class="text-sm text-muted-foreground"
  >
    {m.s8_adv_no_commits_help()}
  </p>{/if}
<AdvancedSection
  title={deferred
    ? m.s8_adv_title_start_commit()
    : mode === "new"
      ? m.s8_adv_title_commit_and_dir()
      : m.s8_adv_title_dir()}
  summary={[
    mode === "new" ? selectedCommit?.shortHash : "",
    deferred ? "" : path,
  ]
    .filter(Boolean)
    .join(" · ")}
>
  {#if mode === "new"}
    <div class="space-y-2">
      <Label for="worktree-base">{m.s8_start_commit_label()}</Label>
      <Select.Root
        type="single"
        bind:value={base}
        disabled={disabled || loading || !hasData || !commitRows.length}
        onValueChange={onbaseedited}
      >
        <Select.Trigger
          id="worktree-base"
          aria-describedby="worktree-base-help"
        >
          {#if selectedCommit}
            {@render commitDetails(selectedCommit)}
          {:else}
            <span class="text-muted-foreground">
              {commitsLoading
                ? m.s8_reading_recent_commits()
                : m.s8_select_commit()}
            </span>
          {/if}
        </Select.Trigger>
        <Select.Content
          class="max-h-[min(24rem,var(--bits-select-content-available-height))]"
        >
          {#each commitRows as item (item.value)}
            <Select.Item
              value={item.value}
              label={item.subject}
              aria-label={commitChoiceLabel(item)}
            >
              {@render commitDetails(item)}
            </Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>
      <p id="worktree-base-help" class="text-xs text-muted-foreground">
        {#if commitsLoading}{m.s8_reading_recent_commits()}{:else}{m.s8_adv_recent_commits_help(
            { count: String(commitsCount) },
          )}{/if}
      </p>
      {#if commitsError}<p role="status" class="text-xs text-warning">
          {commitsError}
        </p>{/if}
    </div>
  {/if}
  {#if !deferred}
    <div class="space-y-2">
      <Label for="worktree-path">{m.s8_new_worktree_full_path_label()}</Label>
      <Input
        id="worktree-path"
        bind:value={path}
        oninput={onpathedited}
        mono
        required
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <p class="text-xs text-muted-foreground">
        {m.s8_new_worktree_path_help()}
      </p>
    </div>
  {/if}
</AdvancedSection>
