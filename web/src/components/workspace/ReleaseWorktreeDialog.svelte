<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import {
    previewRelease,
    releaseWorktree,
  } from "../../domains/sessions/recoverable";
  import type { ReleasePreview, Worktree } from "../../contracts/schemas";
  import BlockerList from "../shared/BlockerList.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { KeyValueSummary } from "$lib/components/ui/key-value-summary";
  import PreviewForm from "./PreviewForm.svelte";
  import PreviewFormActions from "./PreviewFormActions.svelte";
  import PushCheckNotice from "../settings/PushCheckNotice.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

  // Frees the disk space of a linked worktree whose sessions are archived:
  // the directory and its Git registration go, the branch, the archived
  // records and the commit stay so a resume can rebuild it later. The
  // preview lists what would block it and the archived records that stay;
  // ignored files are deleted with the directory and only warned about.
  let {
    worktree,
    onclose,
    onchange,
  }: {
    worktree: Worktree;
    onclose: () => void;
    /** The worktree or its records may have changed. */
    onchange: () => void;
  } = $props();
  const formId = $props.id();
  let preview = $state<ReleasePreview | null>(null);
  let busy = $state(false);
  let error = $state("");

  const commit = $derived(
    (preview?.commit || worktree.head || "").slice(0, 12),
  );
  const hasIgnored = $derived(
    !!preview && (preview.ignored.length > 0 || preview.ignoredMore > 0),
  );

  async function load() {
    busy = true;
    error = "";
    preview = null;
    try {
      preview = await previewRelease(worktree);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
  });

  async function release() {
    if (!preview || preview.blockers.length) return;
    busy = true;
    error = "";
    try {
      await releaseWorktree(worktree, preview.confirmationId);
      onclose();
    } catch (err) {
      error = m.s8_release_failed_retry({ error: errorMessage(err) });
      preview = null;
    } finally {
      busy = false;
      onchange();
    }
  }
</script>

<ResponsiveDialog
  title={m.s8_release_title()}
  description={m.s8_release_desc()}
  drawerDescription={m.s8_release_drawer_desc()}
  {onclose}
  {busy}
  pinDrawerHeader
  drawerFullScreen
  class="h-[min(90dvh,34rem)]"
>
  <PreviewForm
    {formId}
    {busy}
    {error}
    onsubmit={() => void release()}
    {onclose}
  >
    {#snippet summary()}
      <KeyValueSummary
        rows={[
          { label: m.s8_label_path(), value: worktree.path, mono: true },
          {
            label: m.s8_label_branch(),
            value: worktree.branch || m.s8_release_detached_no_branch(),
          },
          {
            label: m.s8_label_saved_commit(),
            value: commit || m.s8_release_unresolvable(),
            mono: true,
            wrap: false,
          },
        ]}
      />
    {/snippet}
    {#if preview}
      {#if preview.blockers.length}
        <BlockerList
          title={m.s8_release_cannot_title()}
          blockers={preview.blockers}
        />
      {/if}
      {#if preview.archived.length}
        <div class="space-y-2 text-sm">
          <p class="font-medium">{m.s8_retained_archived_sessions_heading()}</p>
          <ul class="space-y-1">
            {#each preview.archived as record (record.id)}<li class="break-all">
                {record.title} · {record.cwd}
              </li>{/each}
          </ul>
        </div>
      {/if}
      {#if preview.archived.length && preview.commit}<PushCheckNotice
          push={preview.push}
        />{/if}
      {#if hasIgnored}
        <InlineNotice tone="warning">
          {m.s8_release_ignored_files_warning()}
        </InlineNotice>
      {/if}
      <p class="text-xs text-muted-foreground">
        {m.s8_release_confirmation_ttl_hint()}
      </p>
    {/if}
  </PreviewForm>
  {#snippet footer()}
    <PreviewFormActions
      {formId}
      {busy}
      {onclose}
      confirm={{
        ready: preview !== null,
        disabled: (preview?.blockers.length ?? 0) > 0,
        label: m.s8_release_title(),
        variant: "destructive",
        onreload: load,
      }}
    />
  {/snippet}
</ResponsiveDialog>
