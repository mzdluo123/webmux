<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type { Branches } from "../../contracts/schemas";
  import type { CommitChoice } from "../../domains/workspace/commit-choices";
  import BranchCommandList from "./BranchCommandList.svelte";
  import PickerField from "../shared/PickerField.svelte";
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
      {data || loadError ? placeholder : "正在读取分支…"}
    </span>
  {/if}
{/snippet}

<SegmentedControl.Root
  size="form"
  wrap
  value={mode}
  onValueChange={handleModeChange}
  disabled={busy}
  aria-label="切换目标"
>
  <SegmentedControl.Item value="existing">已有分支</SegmentedControl.Item>
  <SegmentedControl.Item value="new">新建分支</SegmentedControl.Item>
  <SegmentedControl.Item value="detach">分离到提交</SegmentedControl.Item>
</SegmentedControl.Root>

{#if loadError}
  <p role="alert" class="text-sm break-all text-destructive">
    无法读取分支：{loadError}
  </p>
{/if}

{#if mode === "existing"}
  <div class="space-y-2">
    <Label for="switch-branch">目标分支</Label>
    <PickerField
      id="switch-branch"
      bind:open={() => branchOpen, setBranchOpen}
      disabled={busy || !data}
    >
      {#snippet trigger()}{@render pickerFace(
          chosenName,
          "选择本地或远程分支",
        )}{/snippet}
      <BranchCommandList
        label="目标分支"
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
      远程分支会新建同名本地分支并设为其上游。
    </p>
  </div>
{:else if mode === "new"}
  <div class="space-y-3">
    <div class="space-y-2">
      <div class="flex items-center justify-between gap-2">
        <Label for="switch-new-branch">新分支名称</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy || !data}
          onclick={onSuggestName}>换一个</Button
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
      />
      {#if nameTaken}
        <p role="alert" class="text-xs text-destructive">
          已有同名本地分支；请换一个名称，或在“已有分支”中选择它。
        </p>
      {/if}
    </div>
    <div class="space-y-2">
      <Label for="switch-start-point">起点</Label>
      <PickerField
        id="switch-start-point"
        bind:open={() => startOpen, setStartOpen}
        disabled={busy || !data}
      >
        {#snippet trigger()}{@render pickerFace(
            startIsBranch || !startPoint
              ? startPoint
              : `当前提交 ${short(startPoint)}`,
            "选择起点",
          )}{/snippet}
        <BranchCommandList
          label="起点"
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
      <p class="text-xs text-muted-foreground">新分支不设置上游。</p>
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
