<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import {
    previewRebuild,
    rebuildWorktree,
  } from "../../domains/sessions/recoverable";
  import type { HybridRelease, RebuildPreview } from "../../contracts/schemas";
  import BlockerList from "../shared/BlockerList.svelte";
  import { KeyValueSummary } from "$lib/components/ui/key-value-summary";
  import PreviewForm from "./PreviewForm.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

  // The second confirmation before a released record resumes: its worktree
  // comes back at the original path, on its branch when that still points at
  // the saved commit, otherwise detached on the commit. Cancelling changes
  // nothing; the caller resumes once `onrebuilt` runs.
  let {
    id,
    title,
    released,
    onclose,
    onrebuilt,
  }: {
    /** The released record. */
    id: string;
    /** The record's title, naming what resumes. */
    title: string;
    released: HybridRelease;
    onclose: () => void;
    /** The worktree is back; runs instead of `onclose`. */
    onrebuilt: () => void;
  } = $props();
  let preview = $state<RebuildPreview | null>(null);
  let busy = $state(false);
  let error = $state("");

  const commit = $derived((preview?.commit ?? released.commit).slice(0, 12));
  const branch = $derived(preview?.branch ?? released.branch ?? "");
  const mode = $derived.by(() => {
    if (!preview) return "";
    if (preview.mode === "reuse") return m.s8_rebuild_mode_reuse();
    if (preview.mode === "branch") return m.s8_rebuild_mode_branch({ branch });
    if (preview.reason === "branchMissing")
      return m.s8_rebuild_mode_detached_missing({ branch, commit });
    if (preview.reason === "branchMoved")
      return m.s8_rebuild_mode_detached_moved({ branch, commit });
    if (preview.reason === "branchCheckedOut")
      return m.s8_rebuild_mode_detached_checked_out({ branch, commit });
    return m.s8_rebuild_mode_detached_none({ commit });
  });

  async function load() {
    busy = true;
    error = "";
    preview = null;
    try {
      preview = await previewRebuild(id);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
  });

  async function rebuild() {
    if (!preview || preview.blockers.length) return;
    busy = true;
    error = "";
    try {
      await rebuildWorktree(id, preview);
      busy = false;
      onrebuilt();
    } catch (err) {
      error = m.s8_rebuild_failed_retry({ error: errorMessage(err) });
      preview = null;
      busy = false;
    }
  }
</script>

<ResponsiveDialog
  title={m.s8_rebuild_title()}
  description={m.s8_rebuild_desc({ title })}
  drawerDescription={m.s8_rebuild_drawer_desc()}
  {onclose}
  {busy}
>
  <PreviewForm
    {busy}
    submitting={busy && preview !== null}
    {error}
    onsubmit={() => void rebuild()}
    {onclose}
    confirm={{
      ready: preview !== null,
      disabled: (preview?.blockers.length ?? 0) > 0,
      label: m.s8_rebuild_confirm_btn(),
      onreload: load,
    }}
  >
    {#snippet summary()}
      <KeyValueSummary
        rows={[
          {
            label: m.s8_rebuild_original_path(),
            value: preview?.path ?? released.path,
            mono: true,
          },
          {
            label: m.s8_label_saved_commit(),
            value: commit,
            mono: true,
            wrap: false,
          },
        ]}
      />
    {/snippet}
    {#if preview}
      {#if preview.blockers.length}
        <BlockerList
          title={m.s8_rebuild_cannot_title()}
          blockers={preview.blockers}
        />
      {:else}<p class="text-sm">{mode}</p>{/if}
    {/if}
  </PreviewForm>
</ResponsiveDialog>
