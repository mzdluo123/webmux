<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import { onMount } from "svelte";
  import { api } from "../../platform/api";
  import {
    EmptyRequestSchema,
    NoContentSchema,
    RemovalPreviewSchema,
    RemoveWorktreeRequestSchema,
    type ArchivedSession,
    type RemovalPreview,
    type Worktree,
  } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import BlockerList from "../shared/BlockerList.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import PreviewForm from "./PreviewForm.svelte";
  import PreviewFormActions from "./PreviewFormActions.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import PushCheckNotice from "../settings/PushCheckNotice.svelte";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";
  let {
    worktree,
    forgetRecords = [],
    onclose,
    onchange,
    onremoved,
    onhide,
  }: {
    worktree: Worktree;
    /** Archived records the caller deletes after the worktree is gone. */
    forgetRecords?: readonly ArchivedSession[];
    onclose: () => void;
    onchange: () => void;
    /** The worktree was removed; runs before `onclose`. */
    onremoved?: () => void;
    /**
     * Hides the worktree from the home page without deleting anything; when
     * given, the dialog offers it as an alternative to removal.
     */
    onhide?: () => void;
  } = $props();
  const formId = $props.id();
  let preview = $state<RemovalPreview | null>(null);
  let busy = $state(false);
  let error = $state("");
  let force = $state(false);
  let closeSessions = $state(false);
  const canHide = $derived(!!onhide && forgetRecords.length === 0);
  // The records a removal keeps are pinned to the commit, which needs a
  // resolvable HEAD. Records the caller deletes with the worktree are not
  // kept.
  const forgotten = $derived(new Set(forgetRecords.map((record) => record.id)));
  const retainedRecords = $derived(
    preview
      ? preview.archived.filter((record) => !forgotten.has(record.id))
      : [],
  );
  const retained = $derived(
    !!preview && preview.commit !== "" && retainedRecords.length > 0,
  );
  // A force removal also keeps the records of the chat sessions it closes.
  const keepsRecords = $derived(
    !!preview &&
      preview.commit !== "" &&
      (retainedRecords.length > 0 ||
        preview.sessions.some((session) => session.agent !== null)),
  );
  const route = $derived(
    `/repositories/${worktree.repositoryId}/worktrees/${worktree.id}`,
  );
  const dialogDescription = $derived(
    force
      ? forgetRecords.length > 0
        ? m.s8_rm_wt_desc_force_forget()
        : m.s8_rm_wt_desc_force_keep()
      : forgetRecords.length > 0
        ? retainedRecords.length > 0
          ? m.s8_rm_wt_desc_safe_forget_some()
          : m.s8_rm_wt_desc_safe_forget_all()
        : m.s8_rm_wt_desc_safe_keep(),
  );
  async function load() {
    busy = true;
    error = "";
    preview = null;
    force = false;
    closeSessions = false;
    try {
      preview = await api(
        route + "/removal-preview",
        RemovalPreviewSchema,
        "POST",
        {},
        undefined,
        EmptyRequestSchema,
      );
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void load();
  });
  function hide() {
    onhide?.();
    onclose();
  }
  async function remove() {
    if (!preview?.confirmationId) return;
    if (force ? !closeSessions : preview.blockers.length > 0) return;
    busy = true;
    error = "";
    try {
      await api(
        route + "/remove",
        NoContentSchema,
        "POST",
        {
          confirmationId: preview.confirmationId,
          mode: force ? "force" : "safe",
          ...(force
            ? { confirmPath: preview.worktree.path, closeSessions }
            : {}),
        },
        undefined,
        RemoveWorktreeRequestSchema,
      );
      onremoved?.();
      onclose();
    } catch (err) {
      error = m.s8_rm_wt_failed_retry({ error: errorMessage(err) });
      preview = null;
      force = false;
      closeSessions = false;
    } finally {
      busy = false;
      onchange();
    }
  }
</script>

{#snippet recordList(
  records: readonly { id: string; title: string; state: string; cwd: string }[],
)}
  <ul class="space-y-1">
    {#each records as record (record.id)}<li class="break-all">
        {record.title} · {record.state === "closed"
          ? m.s8_record_state_closed()
          : m.s8_record_state_interrupted()} · {record.cwd}
      </li>{/each}
  </ul>
{/snippet}

<ResponsiveDialog
  title={force ? m.s8_rm_wt_title_force() : m.s8_rm_wt_title_safe()}
  description={dialogDescription}
  {onclose}
  {busy}
  pinDrawerHeader
  drawerFullScreen
  class="h-[min(90dvh,34rem)]"
>
  <PreviewForm
    {formId}
    {busy}
    submitting={busy && preview !== null}
    {error}
    onsubmit={() => void remove()}
    {onclose}
  >
    {#snippet summary()}
      <div class="rounded-lg border bg-muted/40 p-3 text-sm">
        <p class="font-medium">
          {worktree.branch ||
            (worktree.detached
              ? `detached · ${worktree.head?.slice(0, 12)}`
              : m.s8_rm_wt_no_branch())}
        </p>
        <p class="mt-1 font-mono break-all whitespace-pre-wrap">
          {worktree.path}
        </p>
        {#if worktree.detached && !retained}<p class="mt-2 text-destructive">
            {m.s8_rm_wt_detached_unprotected_warn()}
          </p>{:else}<p class="mt-2 text-muted-foreground">
            {worktree.detached
              ? m.s8_rm_wt_commit_protected_note()
              : m.s8_rm_wt_branch_kept_note()}
          </p>{/if}
      </div>
    {/snippet}
    {#if preview}
      {#if preview.blockers.length}
        <BlockerList
          title={m.s8_rm_wt_safe_blocked_title()}
          blockers={preview.blockers}
        />
      {:else}<p class="text-sm">
          {preview.worktree.prunable && !preview.worktree.available
            ? m.s8_rm_wt_stale_reg_safe_msg()
            : m.s8_rm_wt_safe_clean_msg()}
        </p>{/if}
      {#if preview.warnings.length}
        <InlineNotice tone="warning">
          <ul class="space-y-1">
            {#each preview.warnings as warning (warning)}<li>
                {warning}
              </li>{/each}
          </ul>
        </InlineNotice>
      {/if}
      {#if preview.sessions.length}
        <WorktreeSessionList
          title={m.s8_rm_wt_associated_sessions_title()}
          sessions={preview.sessions}
        />
      {/if}
      {#if forgetRecords.length}
        <div class="space-y-2 text-sm">
          <p class="font-medium">{m.s8_rm_wt_forget_records_heading()}</p>
          {@render recordList(forgetRecords)}
        </div>
      {/if}
      {#if retainedRecords.length}
        <div class="space-y-2 text-sm">
          <p class="font-medium">
            {retained
              ? m.s8_retained_archived_sessions_heading()
              : m.s8_rm_wt_retained_no_commit_heading()}
          </p>
          {@render recordList(retainedRecords)}
        </div>
      {/if}
      {#if keepsRecords}<PushCheckNotice push={preview.push} />{/if}
      <p class="text-xs text-muted-foreground">
        {m.s8_rm_wt_cd_external_process_note()}
      </p>
      {#if force}
        <Label class="items-start">
          <Checkbox
            bind:checked={closeSessions}
            disabled={busy}
            class="mt-0.5"
          />
          <span>{m.s8_rm_wt_force_ack()}</span>
        </Label>
      {/if}
    {/if}
  </PreviewForm>
  {#snippet footer()}
    <PreviewFormActions {formId} {busy} {onclose}>
      {#snippet actions()}
        {#if !preview}<Button type="button" disabled={busy} onclick={load}
            >{m.s8_reload_preview_btn()}</Button
          >
        {:else if force}
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onclick={() => {
              force = false;
              closeSessions = false;
            }}>{m.s8_rm_wt_back_safe_btn()}</Button
          >
          <Button
            type="submit"
            form={formId}
            wrap
            variant="destructive"
            disabled={busy || !closeSessions || !preview.confirmationId}
            >{m.s8_rm_wt_confirm_force_btn()}</Button
          >
        {:else if !preview.blockers.length}
          <Button
            type="submit"
            form={formId}
            wrap
            disabled={busy || !preview.confirmationId}
            >{m.s8_rm_wt_confirm_safe_btn()}</Button
          >
        {:else if preview.forceAllowed}
          <Button
            type="button"
            variant="destructive"
            disabled={busy}
            onclick={() => {
              force = true;
            }}>{m.s8_rm_wt_enter_force_btn()}</Button
          >
        {/if}
        {#if canHide}
          <Button
            type="button"
            variant="success"
            disabled={busy}
            title={m.s8_rm_wt_hide_only_title()}
            onclick={hide}>{m.s8_rm_wt_hide_only_btn()}</Button
          >
        {/if}
      {/snippet}
    </PreviewFormActions>
  {/snippet}
</ResponsiveDialog>
