<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

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
  title={m.s6_upstream_dialog_title({ name: branchName })}
  description={m.s6_upstream_dialog_desc()}
  drawerDescription={m.s6_upstream_dialog_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-branch-upstream-dialog">
    <div class="rounded-lg border bg-background p-3 text-xs">
      <span class="text-muted-foreground">{m.s6_upstream_current_label()}</span>
      <span class="font-mono text-foreground">
        {currentUpstream || m.s6_upstream_not_set()}
      </span>
    </div>

    {#if remoteBranches.length > 0}
      <div class="space-y-1.5">
        <Label for="git-upstream-select">{m.s6_upstream_select_label()}</Label>
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
          <NativeSelect.Option value="">
            {m.s6_upstream_select_placeholder()}
          </NativeSelect.Option>
          {#each remoteBranches as rb (rb.name)}
            <NativeSelect.Option value={rb.name}>
              {rb.name} ({rb.shortHead})
            </NativeSelect.Option>
          {/each}
        </NativeSelect.Root>
      </div>
    {/if}

    <div class="space-y-1.5">
      <Label for="git-upstream-input">{m.s6_upstream_input_label()}</Label>
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
            {m.s6_upstream_unset_btn()}
          </Button>
        {/if}
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="outline" disabled={submitting} onclick={onClose}>
          {m.s6_cancel()}
        </Button>
        <Button
          variant="default"
          disabled={submitting || !selection.trim()}
          onclick={onConfirmSet}
          data-testid="git-branch-upstream-confirm-btn"
        >
          {submitting ? m.s6_saving_ellipsis() : m.s6_upstream_save_btn()}
        </Button>
      </div>
    </div>
  </div>
</ResponsiveDialog>
