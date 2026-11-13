<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    branchName,
    currentUpstream = "",
    selection,
    remoteBranches = [],
    submitting = false,
    dialogError = "",
    onSelectionChange,
    onConfirmSet,
    onConfirmUnset,
    onClose,
  }: {
    branchName: string;
    currentUpstream?: string;
    selection: string;
    remoteBranches?: readonly GitBranchItem[];
    submitting?: boolean;
    dialogError?: string;
    onSelectionChange: (val: string) => void;
    onConfirmSet: () => void;
    onConfirmUnset: () => void;
    onClose: () => void;
  } = $props();
</script>

<ResponsiveDialog
  title={`设置上游跟踪分支 · ${branchName}`}
  description="绑定或更改本地分支跟踪的本地远程跟踪引用（refs/remotes/*），或取消当前上游关联。"
  drawerDescription="选择或输入远程跟踪引用。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-branch-upstream-dialog">
    <div class="rounded-lg border border-git-border bg-git-panel p-3 text-xs">
      <span class="text-muted-foreground">当前上游跟踪：</span>
      <span class="font-mono text-foreground">
        {currentUpstream || "未设置上游跟踪"}
      </span>
    </div>

    {#if remoteBranches.length > 0}
      <div class="space-y-1.5">
        <Label for="git-upstream-select">从已获取的远程跟踪分支选择</Label>
        <NativeSelect.Root
          id="git-upstream-select"
          value={remoteBranches.some((r) => r.name === selection)
            ? selection
            : ""}
          disabled={submitting}
          onchange={(e) => {
            if (e.currentTarget.value) onSelectionChange(e.currentTarget.value);
          }}
        >
          <NativeSelect.Option value="">-- 选择远程分支 --</NativeSelect.Option>
          {#each remoteBranches as rb (rb.name)}
            <NativeSelect.Option value={rb.name}>
              {rb.name} ({rb.shortHead})
            </NativeSelect.Option>
          {/each}
        </NativeSelect.Root>
      </div>
    {/if}

    <div class="space-y-1.5">
      <Label for="git-upstream-input">目标远程跟踪引用</Label>
      <Input
        id="git-upstream-input"
        mono
        value={selection}
        placeholder="origin/main"
        disabled={submitting}
        oninput={(e) => onSelectionChange(e.currentTarget.value)}
        data-testid="git-branch-upstream-input"
      />
    </div>

    {#if dialogError}
      <InlineNotice tone="danger">{dialogError}</InlineNotice>
    {/if}

    <div class="flex flex-wrap items-center justify-between gap-2 pt-1">
      <div>
        {#if currentUpstream}
          <Button
            variant="destructive-outline"
            disabled={submitting}
            onclick={onConfirmUnset}
            data-testid="git-branch-upstream-unset-btn"
          >
            取消上游跟踪
          </Button>
        {/if}
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="outline" disabled={submitting} onclick={onClose}>
          取消
        </Button>
        <Button
          variant="git-primary"
          disabled={submitting || !selection.trim()}
          onclick={onConfirmSet}
          data-testid="git-branch-upstream-confirm-btn"
        >
          {submitting ? "正在保存…" : "保存上游跟踪"}
        </Button>
      </div>
    </div>
  </div>
</ResponsiveDialog>
