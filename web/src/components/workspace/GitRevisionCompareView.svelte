<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import * as m from "$lib/paraglide/messages.js";
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
      <Label for="git-rev-compare-select">{m.s6_rev_compare_base_label()}</Label
      >
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
              >{m.s6_rev_compare_quick_select()}</NativeSelect.Option
            >
            {#each refs as item (`${item.kind}:${item.name}`)}
              <NativeSelect.Option value={item.head}>
                [{item.kind === "tag"
                  ? m.s6_rev_compare_kind_tag()
                  : item.kind === "remote"
                    ? m.s6_rev_compare_kind_remote()
                    : m.s6_rev_compare_kind_branch()}] {item.name} ({item.shortHead})
              </NativeSelect.Option>
            {/each}
          </NativeSelect.Root>
        {/if}
        <Input
          size="sm"
          mono
          value={baseOid}
          placeholder={m.s6_rev_compare_base_placeholder()}
          aria-label={m.s6_rev_compare_base_aria()}
          oninput={(e) => onSelectBase(e.currentTarget.value)}
          data-testid="git-rev-compare-base-input"
        />
      </div>
    </div>

    <div class="space-y-1.5">
      <Label>{m.s6_rev_compare_target_label()}</Label>
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if hasWorktree}
          <SegmentedControl.Root
            value={targetKind}
            onValueChange={(val) => {
              if (val === "worktree" || val === "rev") onTargetKindChange(val);
            }}
            aria-label={m.s6_rev_compare_target_type_aria()}
          >
            <SegmentedControl.Item value="worktree">
              {m.s6_rev_compare_target_worktree()}
            </SegmentedControl.Item>
            <SegmentedControl.Item value="rev">
              {m.s6_rev_compare_target_rev()}
            </SegmentedControl.Item>
          </SegmentedControl.Root>
        {/if}
        {#if !hasWorktree || targetKind === "rev"}
          <Input
            size="sm"
            mono
            value={targetOid}
            placeholder={m.s6_rev_compare_target_placeholder()}
            aria-label={m.s6_rev_compare_target_aria()}
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
