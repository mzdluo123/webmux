<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconRefresh from "~icons/tabler/refresh";
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import PickerField from "../shared/PickerField.svelte";
  import BranchCommandList from "./BranchCommandList.svelte";
  import WorktreeBaseStatus from "./WorktreeBaseStatus.svelte";

  let {
    mode,
    branch = $bindable(""),
    startBranch,
    disabled = false,
    loading = false,
    hasData = false,
    failed = false,
    refreshing = false,
    baseSource,
    upstream,
    baseSubject,
    lastRefreshedAt,
    refreshWarning,
    localBranches,
    remoteBranches,
    existingBranches,
    onchoosestartbranch,
    onrefreshupstream,
    onsuggestbranch,
    onbranchedited,
  }: {
    mode: "new" | "existing";
    branch: string;
    startBranch: string;
    disabled?: boolean;
    loading?: boolean;
    hasData?: boolean;
    failed?: boolean;
    refreshing?: boolean;
    baseSource: "local" | "remote" | "";
    upstream: string;
    baseSubject: string;
    lastRefreshedAt: Date | null;
    refreshWarning: string;
    localBranches: readonly { name: string }[];
    remoteBranches: readonly { name: string }[];
    existingBranches: readonly { name: string; reason?: string }[];
    onchoosestartbranch: (name: string) => void;
    onrefreshupstream: () => void;
    onsuggestbranch: () => void;
    onbranchedited: () => void;
  } = $props();

  let startBranchPickerOpen = $state(false);
  let startBranchSearch = $state("");
  const setStartBranchPickerOpen = (next: boolean) => {
    startBranchPickerOpen = next;
    if (!next) startBranchSearch = "";
  };
  let branchPickerOpen = $state(false);
  let branchSearch = $state("");
  const setBranchPickerOpen = (next: boolean) => {
    branchPickerOpen = next;
    if (!next) branchSearch = "";
  };

  $effect(() => {
    void mode;
    setStartBranchPickerOpen(false);
    setBranchPickerOpen(false);
  });
</script>

{#if mode === "new"}
  <div class="grid gap-3 sm:grid-cols-2">
    <div class="min-w-0 space-y-2">
      <div class="flex items-center justify-between gap-2">
        <Label for="worktree-start-branch">{m.s8_start_branch_label()}</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!startBranch || refreshing || disabled}
          aria-busy={refreshing}
          onclick={onrefreshupstream}
          ><IconRefresh
            class={refreshing ? "animate-spin motion-reduce:animate-none" : ""}
            aria-hidden="true"
          />{refreshing
            ? m.s8_refreshing_short()
            : m.s8_refresh_upstream()}</Button
        >
      </div>
      <PickerField
        id="worktree-start-branch"
        bind:open={() => startBranchPickerOpen, setStartBranchPickerOpen}
        disabled={disabled || loading || !hasData}
      >
        {#snippet trigger()}
          {#if startBranch}
            <IconGitBranch
              class="size-4 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
            <span class="min-w-0 flex-1 truncate" title={startBranch}
              >{startBranch}</span
            >
          {:else}
            <span class="text-sm text-muted-foreground">
              {loading
                ? m.s8_reading_local_branches()
                : m.s8_select_start_branch()}
            </span>
          {/if}
        {/snippet}
        <BranchCommandList
          label={m.s8_start_branch_label()}
          branches={[...localBranches]}
          remoteBranches={[...remoteBranches]}
          selected={startBranch}
          {loading}
          {failed}
          {disabled}
          bind:search={startBranchSearch}
          onselect={(name) => {
            onchoosestartbranch(name);
            setStartBranchPickerOpen(false);
          }}
        />
      </PickerField>
      <WorktreeBaseStatus
        {refreshing}
        {startBranch}
        {baseSource}
        {upstream}
        {baseSubject}
        {lastRefreshedAt}
        {refreshWarning}
      />
    </div>
    <div class="min-w-0 space-y-2">
      <div class="flex items-center justify-between gap-2">
        <Label for="worktree-branch">{m.s8_new_branch_name_label()}</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || !hasData || loading}
          onclick={onsuggestbranch}>{m.s8_suggest_another()}</Button
        >
      </div>
      <Input
        id="worktree-branch"
        disabled={disabled || loading || !hasData}
        bind:value={branch}
        oninput={onbranchedited}
        required
        placeholder="feature/my-task"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
    </div>
  </div>
{:else}
  <div class="space-y-2">
    <Label for="existing-branch">{m.s8_existing_local_branch_label()}</Label>
    <PickerField
      id="existing-branch"
      bind:open={() => branchPickerOpen, setBranchPickerOpen}
      disabled={disabled || loading || !hasData}
    >
      {#snippet trigger()}
        {#if branch}
          <IconGitBranch
            class="size-4 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <span class="min-w-0 flex-1 truncate" title={branch}>{branch}</span>
        {:else}
          <span class="text-sm text-muted-foreground">
            {loading
              ? m.s8_reading_local_branches()
              : m.s8_select_local_branch()}
          </span>
        {/if}
      {/snippet}
      <BranchCommandList
        label={m.s8_existing_local_branch_label()}
        branches={[...existingBranches]}
        selected={branch}
        {loading}
        {failed}
        {disabled}
        bind:search={branchSearch}
        onselect={(name) => ((branch = name), setBranchPickerOpen(false))}
      />
    </PickerField>
  </div>
{/if}
