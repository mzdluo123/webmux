<script lang="ts">
  import { tick } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { m } from "$lib/paraglide/messages.js";
  import { api } from "../../platform/api";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import { errorMessage } from "../../platform/error";
  import {
    BatchRemovalPreviewRequestSchema,
    BatchRemovalPreviewSchema,
    BatchRemoveResultSchema,
    BatchRemoveWorktreesRequestSchema,
    MAX_BATCH_WORKTREES,
    type ArchivedSession,
    type BatchRemovalPreview,
    type BatchRemoveResult,
    type HomeWorktrees,
    type Repository,
    type SessionInfo,
    type WorktreePins,
  } from "../../contracts/schemas";
  import {
    buildWorktreeBatchEntries,
    countWorktreeBatchCategories,
    filterWorktreeBatchEntries,
    groupBatchPreviewItems,
    selectableSafeEntries,
    staleRegistrations,
    type WorktreeBatchFilter,
  } from "../../domains/workspace/worktree-batch";
  import { worktreePins } from "../../domains/workspace/worktree-pins.svelte";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import ManageWorktreesPreview from "./ManageWorktreesPreview.svelte";
  import ManageWorktreesResultNotice from "./ManageWorktreesResultNotice.svelte";
  import ManageWorktreesSelectList from "./ManageWorktreesSelectList.svelte";
  import RepositoryWorktreesToolbar from "./RepositoryWorktreesToolbar.svelte";

  // The repository panel's worktrees: every worktree of the repository,
  // searchable and filtered by how safely it can be removed. Browsing opens
  // a worktree's details; "批量清理" turns the list into a selection, then
  // a server preview, then per-item results. "清理失效登记" preselects the
  // stale registrations and goes straight to that preview. Removal happens
  // only here in the panel, so the rows' menus carry no delete entry.
  let {
    repository,
    sessions,
    archived,
    home,
    pins = worktreePins.value,
    busy = $bindable(false),
    onopen,
    onswitch,
    onoperation,
    ongit,
    onchange,
    onremoved,
  }: {
    repository: Repository;
    sessions: readonly SessionInfo[];
    archived: readonly ArchivedSession[];
    home: HomeWorktrees;
    pins?: WorktreePins;
    /** A preview or removal request is in flight. */
    busy?: boolean;
    /** Opens a worktree's details inside the panel. */
    onopen: (worktreeId: string) => void;
    /** Opens the switch of a worktree's branch inside the panel. */
    onswitch: (worktreeId: string) => void;
    /** Opens a branch operation inside the panel. */
    onoperation: (operation: BranchOperation) => void;
    ongit?: (worktreeId: string, view?: "commit") => void;
    onchange: () => void;
    onremoved?: (removedIds: string[]) => void;
  } = $props();

  type Stage = "browse" | "select" | "preview" | "result";
  let stage = $state<Stage>("browse");
  let query = $state("");
  let statusFilter = $state<WorktreeBatchFilter>("all");
  const selected = new SvelteSet<string>();
  let error = $state("");
  let done = $state("");
  let preview = $state<BatchRemovalPreview | null>(null);
  let includeForce = $state(false);
  let lastResult = $state<BatchRemoveResult | null>(null);
  let stepBody = $state<HTMLDivElement | null>(null);
  let stepHeading = $state<HTMLHeadingElement | null>(null);
  let selectionAnchor: { id: string; offset: number } | undefined;
  let selectionScrollTop = 0;

  function viewport() {
    return stepBody?.closest<HTMLElement>('[data-slot="scroll-area-viewport"]');
  }

  function checkbox(id: string) {
    return stepBody?.querySelector<HTMLElement>(
      `[data-worktree-checkbox="${CSS.escape(id)}"]`,
    );
  }

  async function showReviewStep(next: "preview" | "result") {
    stage = next;
    await tick();
    if (!stepHeading || stage !== next) return;
    stepHeading.focus({ preventScroll: true });
    stepHeading.scrollIntoView({ block: "start", inline: "nearest" });
  }

  async function returnToSelection() {
    error = "";
    stage = "select";
    await tick();
    const scroller = viewport();
    if (stage !== "select" || !scroller) return;
    const anchor = selectionAnchor;
    const target = anchor && checkbox(anchor.id);
    if (target && !target.matches(":disabled,[aria-disabled=true]")) {
      target.focus({ preventScroll: true });
      scroller.scrollTop +=
        target.getBoundingClientRect().top -
        scroller.getBoundingClientRect().top -
        anchor.offset;
    } else {
      stepBody?.focus({ preventScroll: true });
      scroller.scrollTop = selectionScrollTop;
    }
  }

  const entries = $derived(
    buildWorktreeBatchEntries(repository, home, sessions, archived, pins),
  );
  const counts = $derived(countWorktreeBatchCategories(entries));
  const selectableSafe = $derived(selectableSafeEntries(entries));
  const stale = $derived(
    staleRegistrations(entries)
      .slice(0, MAX_BATCH_WORKTREES)
      .map((entry) => entry.tree.id),
  );
  const filteredEntries = $derived(
    filterWorktreeBatchEntries(entries, statusFilter, query),
  );

  // Selected IDs kept in the stable repository order; protected or vanished
  // entries are ignored if a background refresh changed the snapshot.
  const selectedIds = $derived(
    entries
      .filter(
        (entry) =>
          entry.category !== "protected" && selected.has(entry.tree.id),
      )
      .map((entry) => entry.tree.id),
  );

  const previewGroups = $derived(
    preview
      ? groupBatchPreviewItems(preview.items)
      : { safe: [], blocked: [], unsupported: [] },
  );
  const actionCount = $derived(
    previewGroups.safe.length +
      (includeForce ? previewGroups.blocked.length : 0),
  );

  const resultSummary = $derived.by(() => {
    const results = lastResult?.results ?? [];
    return {
      removed: results.filter((r) => r.status === "removed").length,
      skipped: results.filter((r) => r.status === "skipped").length,
      failed: results.filter((r) => r.status === "failed"),
    };
  });

  const heading = $derived(
    stage === "preview"
      ? includeForce
        ? m.s8_batch_heading_force()
        : m.s8_batch_heading_confirm()
      : stage === "result"
        ? m.s8_batch_heading_partial()
        : "",
  );
  const description = $derived(
    stage === "preview"
      ? includeForce
        ? m.s8_batch_desc_force()
        : m.s8_batch_desc_safe()
      : m.s8_batch_desc_partial(),
  );

  function toggleWorktree(id: string, checked: boolean) {
    error = "";
    const target = checkbox(id);
    const scroller = viewport();
    if (target && scroller) {
      selectionAnchor = {
        id,
        offset:
          target.getBoundingClientRect().top -
          scroller.getBoundingClientRect().top,
      };
    }
    if (checked) {
      if (selectedIds.length >= MAX_BATCH_WORKTREES && !selected.has(id)) {
        error = m.s8_batch_err_max_select({ max: String(MAX_BATCH_WORKTREES) });
        return;
      }
      selected.add(id);
    } else {
      selected.delete(id);
    }
  }

  function selectAllSafe() {
    error = "";
    selected.clear();
    let count = 0;
    for (const entry of selectableSafe) {
      if (count >= MAX_BATCH_WORKTREES) {
        error = m.s8_batch_err_capped_safe({
          max: String(MAX_BATCH_WORKTREES),
        });
        break;
      }
      selected.add(entry.tree.id);
      count++;
    }
  }

  function startSelecting() {
    selectionAnchor = undefined;
    selectionScrollTop = 0;
    error = "";
    done = "";
    stage = "select";
  }

  function stopSelecting() {
    error = "";
    selected.clear();
    stage = "browse";
  }

  // Selects exactly the stale registrations and previews them; a failed
  // preview leaves the selection to review.
  async function pruneStale() {
    selectionAnchor = undefined;
    selectionScrollTop = 0;
    done = "";
    selected.clear();
    for (const id of stale) selected.add(id);
    await loadPreview(stale);
    if (stage !== "preview") stage = "select";
  }

  async function loadPreview(ids: readonly string[] = selectedIds) {
    if (ids.length === 0) return;
    if (stage === "select") selectionScrollTop = viewport()?.scrollTop ?? 0;
    busy = true;
    error = "";
    includeForce = false;
    try {
      preview = await api(
        `/repositories/${repository.id}/worktrees/batch-removal-preview`,
        BatchRemovalPreviewSchema,
        "POST",
        { worktreeIds: [...ids] },
        undefined,
        BatchRemovalPreviewRequestSchema,
      );
      lastResult = null;
      void showReviewStep("preview");
    } catch (err) {
      error = errorMessage(err);
      onchange();
    } finally {
      busy = false;
    }
  }

  async function executeRemove() {
    if (!preview) return;
    busy = true;
    error = "";
    try {
      const res = await api(
        `/repositories/${repository.id}/worktrees/batch-remove`,
        BatchRemoveResultSchema,
        "POST",
        {
          confirmationId: preview.confirmationId,
          includeForce,
          closeSessions: includeForce,
        },
        undefined,
        BatchRemoveWorktreesRequestSchema,
      );
      const removed = res.results
        .filter((item) => item.status === "removed")
        .map((item) => item.worktreeId);
      const failed = res.results.filter((item) => item.status === "failed");
      if (removed.length > 0) onremoved?.(removed);
      onchange();
      preview = null;
      includeForce = false;
      selected.clear();
      if (failed.length === 0) {
        // Everything asked for is done: back to the refreshed list.
        done = m.s8_batch_done_removed({ count: String(removed.length) });
        stage = "browse";
        return;
      }
      for (const item of failed) selected.add(item.worktreeId);
      lastResult = res;
      void showReviewStep("result");
    } catch (err) {
      error = m.s8_batch_err_reload_preview({ error: errorMessage(err) });
      preview = null;
      includeForce = false;
      stage = "select";
      onchange();
    } finally {
      busy = false;
    }
  }
</script>

<div bind:this={stepBody} class="space-y-4" tabindex="-1">
  {#if stage === "browse" || stage === "select"}
    <RepositoryWorktreesToolbar
      {stage}
      bind:query
      bind:statusFilter
      {counts}
      selectableSafeCount={selectableSafe.length}
      selectedCount={selectedIds.length}
      totalCount={entries.length}
      staleCount={stale.length}
      {busy}
      onselectallsafe={selectAllSafe}
      onclearselection={() => {
        error = "";
        selected.clear();
      }}
      onstopselecting={stopSelecting}
      onprunestale={() => void pruneStale()}
      onstartselecting={startSelecting}
    />

    {#if done}<InlineNotice tone="success">{done}</InlineNotice>{/if}

    <ManageWorktreesSelectList
      {repository}
      {pins}
      {stage}
      {entries}
      {filteredEntries}
      {selectedIds}
      {busy}
      ontoggle={toggleWorktree}
      {onopen}
      {onswitch}
      {onoperation}
      {ongit}
    />
  {:else}
    <div class="space-y-1">
      <h3 bind:this={stepHeading} tabindex="-1" class="text-sm font-medium">
        {heading}
      </h3>
      <p class="text-xs text-muted-foreground">{description}</p>
    </div>
    {#if stage === "preview" && preview}
      {#if busy}
        <p role="status" class="text-sm">{m.s8_processing_no_resubmit()}</p>
      {/if}
      <ManageWorktreesPreview groups={previewGroups} bind:includeForce {busy} />
    {:else if stage === "result" && lastResult}
      <ManageWorktreesResultNotice
        removed={resultSummary.removed}
        skipped={resultSummary.skipped}
        failed={resultSummary.failed}
      />
    {/if}
  {/if}

  {#if error}
    <p role="alert" class="text-sm break-all text-destructive">{error}</p>
  {/if}

  {#if stage !== "browse"}
    <div class="flex flex-wrap justify-end gap-2">
      {#if stage === "select"}
        <Button
          type="button"
          disabled={busy ||
            selectedIds.length === 0 ||
            selectedIds.length > MAX_BATCH_WORKTREES}
          onclick={() => void loadPreview()}
        >
          {busy
            ? m.s8_loading_preview()
            : m.s8_wt_next_preview({ count: String(selectedIds.length) })}
        </Button>
      {:else if stage === "preview" && preview}
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onclick={() => {
            preview = null;
            includeForce = false;
            void returnToSelection();
          }}
        >
          {m.s8_wt_back_reselect()}
        </Button>
        {#if includeForce && previewGroups.blocked.length > 0}
          <Button
            type="button"
            variant="destructive"
            disabled={busy || actionCount === 0}
            onclick={() => void executeRemove()}
          >
            {busy
              ? m.s8_wt_closing_and_removing()
              : m.s8_wt_close_and_remove({ count: String(actionCount) })}
          </Button>
        {:else}
          <Button
            type="button"
            disabled={busy || previewGroups.safe.length === 0}
            onclick={() => void executeRemove()}
          >
            {busy
              ? m.s8_wt_cleaning()
              : m.s8_wt_confirm_safe_remove({
                  count: String(previewGroups.safe.length),
                })}
          </Button>
        {/if}
      {:else if stage === "result" && lastResult}
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onclick={() => {
            lastResult = null;
            void returnToSelection();
          }}
        >
          {m.s8_wt_back_to_list()}
        </Button>
        <Button
          type="button"
          disabled={busy || selected.size === 0}
          onclick={() => void loadPreview([...selected])}
        >
          {busy
            ? m.s8_loading_preview()
            : m.s8_wt_repreview_retry({ count: String(selected.size) })}
        </Button>
      {/if}
    </div>
  {/if}
</div>
