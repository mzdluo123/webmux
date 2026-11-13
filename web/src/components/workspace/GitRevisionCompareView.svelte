<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";

  let {
    refs = [],
    baseOid,
    targetKind,
    targetOid,
    hasWorktree = false,
    diffReader,
    onSelectBase,
    onTargetKindChange,
    onTargetOidChange,
  }: {
    refs?: readonly GitBranchItem[];
    baseOid: string;
    targetKind: "worktree" | "rev";
    targetOid: string;
    hasWorktree?: boolean;
    diffReader?: GitDiffPanelReader;
    onSelectBase: (oid: string, label?: string) => void;
    onTargetKindChange: (kind: "worktree" | "rev") => void;
    onTargetOidChange: (oid: string) => void;
  } = $props();
</script>

<div
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background"
  data-testid="git-revision-compare-view"
>
  <div
    class="grid grid-cols-1 gap-2.5 border-b bg-background p-3 text-xs sm:grid-cols-2"
  >
    <div class="space-y-1.5">
      <Label for="git-rev-compare-select">选择基准分支 / 标签 / 提交</Label>
      <div class="flex min-w-0 items-center gap-1.5">
        {#if refs.length > 0}
          <NativeSelect.Root
            id="git-rev-compare-select"
            size="sm"
            value={refs.some((r) => r.head === baseOid) ? baseOid : ""}
            onchange={(e) => {
              const chosen = refs.find((r) => r.head === e.currentTarget.value);
              if (chosen) onSelectBase(chosen.head, chosen.name);
            }}
          >
            <NativeSelect.Option value=""
              >-- 快速选择引用 --</NativeSelect.Option
            >
            {#each refs as item (`${item.kind}:${item.name}`)}
              <NativeSelect.Option value={item.head}>
                [{item.kind === "tag"
                  ? "标签"
                  : item.kind === "remote"
                    ? "远程"
                    : "分支"}] {item.name} ({item.shortHead})
              </NativeSelect.Option>
            {/each}
          </NativeSelect.Root>
        {/if}
        <Input
          size="sm"
          mono
          value={baseOid}
          placeholder="基准提交 OID…"
          aria-label="基准提交 OID"
          oninput={(e) => onSelectBase(e.currentTarget.value)}
          data-testid="git-rev-compare-base-input"
        />
      </div>
    </div>

    <div class="space-y-1.5">
      <Label>对比右侧目标</Label>
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if hasWorktree}
          <SegmentedControl.Root
            value={targetKind}
            onValueChange={(val) => {
              if (val === "worktree" || val === "rev") onTargetKindChange(val);
            }}
            aria-label="对比右侧目标类型"
          >
            <SegmentedControl.Item value="worktree">
              当前工作区文件
            </SegmentedControl.Item>
            <SegmentedControl.Item value="rev">指定提交</SegmentedControl.Item>
          </SegmentedControl.Root>
        {/if}
        {#if !hasWorktree || targetKind === "rev"}
          <Input
            size="sm"
            mono
            value={targetOid}
            placeholder="目标提交 OID…"
            aria-label="目标提交 OID"
            oninput={(e) => onTargetOidChange(e.currentTarget.value)}
            data-testid="git-rev-compare-target-input"
          />
        {/if}
      </div>
    </div>
  </div>

  <div class="min-h-0 min-w-0 flex-1">
    <GitLogDiffPanel logDiff={diffReader} presentation="docked" />
  </div>
</div>
