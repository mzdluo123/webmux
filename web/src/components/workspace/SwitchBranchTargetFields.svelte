<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import type { Branches } from "../../contracts/schemas";
  import type { CommitChoice } from "../../domains/workspace/commit-choices";
  import BranchCommandList from "./BranchCommandList.svelte";
  import { PickerField } from "$lib/components/ui/picker-field";
  import SwitchBranchCommitSelect from "./SwitchBranchCommitSelect.svelte";

  export type SwitchTargetMode = "existing" | "new" | "detach";

  interface BranchItem {
    name: string;
    reason?: string;
  }

  let {
    mode,
    busy = false,
    data,
    loadError = "",
    chosenName = "",
    localItems,
    remoteItems,
    localBranchList,
    remoteBranchList,
    newName = $bindable(""),
    nameTaken = false,
    startPoint = $bindable(""),
    startIsBranch = false,
    commit = $bindable(""),
    commitRows,
    commitsRead = false,
    commitsError = "",
    onModeChange,
    onSelectBranch,
    onSuggestName,
  }: {
    mode: SwitchTargetMode;
    busy?: boolean;
    data: Branches | null;
    loadError?: string;
    chosenName?: string;
    localItems: readonly BranchItem[];
    remoteItems: readonly BranchItem[];
    localBranchList: readonly { name: string }[];
    remoteBranchList: readonly { name: string }[];
    newName?: string;
    nameTaken?: boolean;
    startPoint?: string;
    startIsBranch?: boolean;
    commit?: string;
    commitRows: readonly CommitChoice[];
    commitsRead?: boolean;
    commitsError?: string;
    onModeChange: (next: string) => void;
    onSelectBranch: (name: string) => void;
    onSuggestName: () => void;
  } = $props();

  let branchOpen = $state(false);
  let branchSearch = $state("");
  const setBranchOpen = (next: boolean) => (
    (branchOpen = next),
    !next && (branchSearch = "")
  );

  let startOpen = $state(false);
  let startSearch = $state("");
  const setStartOpen = (next: boolean) => (
    (startOpen = next),
    !next && (startSearch = "")
  );

  const short = (hash: string | null | undefined) => hash?.slice(0, 8) ?? "";

  function handleModeChange(next: string) {
    setBranchOpen(false);
    setStartOpen(false);
    onModeChange(next);
  }
</script>

{#snippet pickerFace(value: string, placeholder: string)}
  {#if value}
    <IconGitBranch
      class="size-4 shrink-0 text-muted-foreground"
      aria-hidden="true"
    />
    <span class="min-w-0 flex-1 truncate" title={value}>{value}</span>
  {:else}
    <span class="text-sm text-muted-foreground">
      {data || loadError ? placeholder : m.s8_reading_branches()}
    </span>
  {/if}
{/snippet}

<SegmentedControl
  size="form"
  wrap
  value={mode}
  onValueChange={handleModeChange}
  disabled={busy}
  aria-label={m.s8_switch_target_aria()}
>
  <SegmentedControlItem value="existing"
    >{m.s8_switch_mode_existing()}</SegmentedControlItem
  >
  <SegmentedControlItem value="new"
    >{m.s8_switch_mode_new()}</SegmentedControlItem
  >
  <SegmentedControlItem value="detach"
    >{m.s8_switch_mode_detach()}</SegmentedControlItem
  >
</SegmentedControl>

{#if loadError}
  <p role="alert" class="text-sm break-all text-destructive">
    {m.s8_err_load_branches({ error: loadError })}
  </p>
{/if}

<div class="min-h-52">
  {#if mode === "existing"}
    <div class="space-y-2">
      <Label for="switch-branch">{m.s8_target_branch_label()}</Label>
      <PickerField
        id="switch-branch"
        bind:open={() => branchOpen, setBranchOpen}
        disabled={busy || !data}
      >
        {#snippet trigger()}{@render pickerFace(
            chosenName,
            m.s8_select_local_or_remote_branch(),
          )}{/snippet}
        <BranchCommandList
          label={m.s8_target_branch_label()}
          branches={[...localItems]}
          remoteBranches={[...remoteItems]}
          selected={chosenName}
          loading={!data && !loadError}
          failed={!!loadError}
          disabled={busy}
          bind:search={branchSearch}
          onselect={(name) => {
            onSelectBranch(name);
            setBranchOpen(false);
          }}
        />
      </PickerField>
      <p class="text-xs text-muted-foreground">
        {m.s8_remote_branch_creates_local_help()}
      </p>
    </div>
  {:else if mode === "new"}
    <div class="space-y-3">
      <div class="space-y-2">
        <div class="flex items-center justify-between gap-2">
          <Label for="switch-new-branch">{m.s8_new_branch_name_label()}</Label>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy || !data}
            onclick={onSuggestName}>{m.s8_suggest_another()}</Button
          >
        </div>
        <Input
          id="switch-new-branch"
          bind:value={newName}
          disabled={busy}
          placeholder="feature/my-task"
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
          aria-invalid={nameTaken}
          aria-describedby={nameTaken ? "switch-new-branch-error" : undefined}
        />
        <p
          id="switch-new-branch-error"
          role={nameTaken ? "alert" : undefined}
          class:invisible={!nameTaken}
          class="text-xs text-destructive"
        >
          {m.s8_err_local_branch_name_taken()}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="switch-start-point">{m.s8_start_point_label()}</Label>
        <PickerField
          id="switch-start-point"
          bind:open={() => startOpen, setStartOpen}
          disabled={busy || !data}
        >
          {#snippet trigger()}{@render pickerFace(
              startIsBranch || !startPoint
                ? startPoint
                : m.s8_current_commit_hash({ hash: short(startPoint) }),
              m.s8_select_start_point(),
            )}{/snippet}
          <BranchCommandList
            label={m.s8_start_point_label()}
            branches={[...localBranchList]}
            remoteBranches={[...remoteBranchList]}
            selected={startPoint}
            loading={!data && !loadError}
            failed={!!loadError}
            disabled={busy}
            bind:search={startSearch}
            onselect={(name) => ((startPoint = name), setStartOpen(false))}
          />
        </PickerField>
        <p class="text-xs text-muted-foreground">
          {m.s8_new_branch_no_upstream_help()}
        </p>
      </div>
    </div>
  {:else}
    <SwitchBranchCommitSelect
      bind:commit
      {commitRows}
      {commitsRead}
      {commitsError}
      {busy}
    />
  {/if}
</div>
