<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import {
    remoteBranchBases,
    suggestWorktreeBranch,
  } from "../../domains/workspace/branch-name";
  import {
    branchTips,
    commitChoices,
    fetchRepositoryBranches,
    fetchRepositoryCommits,
    formatCommitLoadError,
    isAbortError,
    refreshRepositoryBranchBase,
    resolveInitialStartBranch,
    resolveRefreshedLocalHead,
    resolveSelectedStartBranchState,
    resolveWorktreePresetSelection,
    type CommitTip,
  } from "../../domains/workspace/commit-choices";
  import { LatestRequest } from "../../hooks/shared/remote-request.svelte";
  import type { WorktreePreset } from "../../domains/sessions/worktree-preset";
  import {
    BranchesSchema,
    CommitsSchema,
    type Repository,
  } from "../../contracts/schemas";
  import WorktreeAdvancedFields from "./WorktreeAdvancedFields.svelte";
  import WorktreeBranchFields from "./WorktreeBranchFields.svelte";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { untrack } from "svelte";
  import type { z } from "zod";
  let {
    repository,
    deferred = false,
    path = $bindable(""),
    mode = $bindable<"new" | "existing">("new"),
    branch = $bindable(""),
    base = $bindable(""),
    startBranch = $bindable(""),
    disabled = false,
    preferredStartBranch,
    preset,
  }: {
    repository: Repository;
    /** Handoff reserves a fresh branch and assigns its directory on send. */
    deferred?: boolean;
    path: string;
    mode: "new" | "existing";
    branch: string;
    base: string;
    /** The local branch a new branch starts from. */
    startBranch?: string;
    disabled?: boolean;
    /** The start branch to use when it is a local branch, else the default. */
    preferredStartBranch?: string;
    /** Initial values, applied once when the branches are first read. */
    preset?: WorktreePreset;
  } = $props();
  // The preset sets the mode at once so the form never shows the other one
  // first; its branches wait for the list.
  let pendingPreset = untrack(() => preset);
  // A status refresh for the same repository must not reinitialize its draft.
  const repositoryId = $derived(repository.id);
  if (pendingPreset) mode = pendingPreset.mode;
  let data = $state<z.infer<typeof BranchesSchema> | null>(null);
  let error = $state("");
  let loading = $state(false);
  let refreshing = $state(false);
  let refreshWarning = $state("");
  let upstream = $state("");
  let baseSource = $state<"local" | "remote" | "">("");
  let baseSubject = $state("");
  let lastRefreshedAt = $state<Date | null>(null);
  const refreshRequests = new LatestRequest();
  let baseEditVersion = 0;
  let pathEdited = $state(false);
  let branchEdited = $state(false);
  let commits = $state<z.infer<typeof CommitsSchema>["commits"]>([]);
  let commitsLoading = $state(false);
  let commitsError = $state("");
  const commitRequests = new LatestRequest();
  // Tips the history was read from; their rows carry these tags.
  let commitTips = $state<CommitTip[]>([]);
  const remoteBranches = $derived(
    data
      ? remoteBranchBases(
          data.bases,
          data.branches.map((item) => item.name),
        )
      : [],
  );
  const commitRows = $derived(
    commitChoices(commits, { tips: commitTips, base, baseSubject }),
  );
  const selectedCommit = $derived(
    commitRows.find((item) => item.value === base),
  );

  function clearCommits() {
    commitRequests.cancel();
    commitsLoading = false;
    commitsError = "";
  }

  // Lists the history reachable from the tips, merged newest first. Later
  // calls supersede earlier ones.
  function loadCommits(tips: CommitTip[]) {
    const request = commitRequests.begin();
    commitTips = tips;
    commitsLoading = true;
    commitsError = "";
    void fetchRepositoryCommits(repository.id, tips, request.signal)
      .then((value) => {
        if (request.current()) commits = value.commits;
      })
      .catch((err: unknown) => {
        if (!request.reportable()) return;
        commits = [];
        commitsError = formatCommitLoadError(err);
      })
      .finally(() => {
        if (request.current()) commitsLoading = false;
      });
  }

  const suggestedBranch = (avoid?: string) =>
    data
      ? suggestWorktreeBranch({
          branchNames: data.branches.map((item) => item.name),
          worktreePaths: repository.worktrees.map((tree) => tree.path),
          avoid,
        })
      : "";
  function fillSuggestedBranch() {
    if (mode === "new" && !branchEdited && !branch && data)
      branch = suggestedBranch();
  }

  function clearRefresh() {
    refreshRequests.cancel();
    refreshing = false;
    refreshWarning = "";
    upstream = "";
    lastRefreshedAt = null;
  }

  function refreshUpstream(branchName: string) {
    const request = refreshRequests.begin();
    const editVersion = baseEditVersion;
    const repositoryID = repository.id;
    refreshing = true;
    refreshWarning = "";
    void refreshRepositoryBranchBase(repositoryID, branchName, request.signal)
      .then((value) => {
        if (
          !request.current() ||
          mode !== "new" ||
          repository.id !== repositoryID ||
          startBranch !== branchName
        )
          return;
        upstream = value.upstream;
        refreshWarning = value.fetchWarning;
        lastRefreshedAt = value.baseSource === "remote" ? new Date() : null;
        const localHead = resolveRefreshedLocalHead(value, data, branchName);
        if (editVersion === baseEditVersion) {
          base = value.base;
          baseSource = value.baseSource;
          baseSubject = value.baseSubject;
        }
        loadCommits(branchTips(localHead, value.upstreamHead));
        if (!value.fetchWarning)
          void fetchRepositoryBranches(repositoryID, request.signal)
            .then((next) => {
              if (request.current() && repository.id === repositoryID)
                data = next;
            })
            .catch(() => {});
      })
      .catch((err: unknown) => {
        if (!request.reportable() || isAbortError(err)) return;
        refreshWarning = m.s8_upstream_refresh_failed_keep_base();
        baseSource = "";
        upstream = "";
        lastRefreshedAt = null;
      })
      .finally(() => {
        if (request.current()) refreshing = false;
      });
  }

  function chooseStartBranch(name: string) {
    startBranch = name;
    clearRefresh();
    const resolved = resolveSelectedStartBranchState(
      name,
      data,
      remoteBranches,
    );
    base = resolved.base;
    baseSubject = resolved.baseSubject;
    if (resolved.kind === "remote") {
      baseSource = "remote";
      upstream = resolved.upstream;
      loadCommits(resolved.tips);
      refreshUpstream(name);
      return;
    }
    if (resolved.kind === "local") {
      baseSource = "local";
      loadCommits(resolved.tips);
      refreshUpstream(name);
      return;
    }
    baseSource = "";
    if (resolved.tips.length) loadCommits(resolved.tips);
    else clearCommits();
  }

  $effect(() => {
    const id = repositoryId;
    let active = true;
    clearRefresh();
    clearCommits();
    commits = [];
    data = null;
    startBranch = "";
    baseSource = "";
    baseSubject = "";
    error = "";
    loading = true;
    pathEdited = false;
    branchEdited = false;
    branch = pendingPreset?.mode === "new" ? (pendingPreset.name ?? "") : "";
    base = pendingPreset?.base ?? "";
    void fetchRepositoryBranches(id)
      .then((value) => {
        if (!active) return;
        data = value;
        base = value.defaultBase || "";
        baseSubject = value.defaultBaseSubject;
        const initial = pendingPreset;
        pendingPreset = undefined;
        const applied = initial
          ? resolveWorktreePresetSelection(initial, value)
          : null;
        if (applied) {
          branch = applied.branch;
          branchEdited = applied.branchEdited;
        }
        fillSuggestedBranch();
        startBranch = resolveInitialStartBranch(
          value,
          remoteBranches,
          initial,
          preferredStartBranch,
        );
        if (mode === "new" && initial?.base) {
          base = initial.base;
          baseSubject = m.s8_saved_start_commit();
          loadCommits([{ hash: base, tag: m.s8_saved_start_commit() }]);
        } else if (mode === "new") chooseStartBranch(startBranch);
      })
      .catch((err) => active && (error = errorMessage(err)))
      .finally(() => active && (loading = false));
    return () => {
      active = false;
      clearRefresh();
      clearCommits();
    };
  });
  $effect(() => {
    if (!pathEdited)
      path = data
        ? `${data.defaultParent}/${branch.split("/").at(-1) || ""}`
        : "";
  });
  const mappedLocalBranches = $derived(
    (data?.branches ?? []).map((item) => ({ name: item.name })),
  );
  const mappedExistingBranches = $derived(
    (data?.branches ?? []).map((item) => ({
      name: item.name,
      reason: item.worktreePaths.length
        ? m.s8_branch_occupied_by_worktree()
        : undefined,
    })),
  );
  const mappedRemoteBranches = $derived(
    remoteBranches.map((item) => ({ name: item.name })),
  );
  function handleModeChange(next: string | undefined) {
    if (next === "new" && mode !== "new") {
      mode = "new";
      branchEdited = false;
      branch = "";
      fillSuggestedBranch();
      chooseStartBranch(startBranch);
    } else if (next === "existing" && mode !== "existing") {
      mode = "existing";
      clearRefresh();
      clearCommits();
      baseSource = "";
      branch = "";
    }
  }
  const ready = $derived(
    !!data && !loading && !!branch && !!path && (mode === "existing" || !!base),
  );
  export const isReady = () => ready;
</script>

<div class="space-y-3 rounded-lg border p-3">
  {#if !deferred}
    <SegmentedControl
      value={mode}
      onValueChange={handleModeChange}
      aria-label={m.s8_worktree_branch_aria()}
    >
      <SegmentedControlItem value="new"
        >{m.s8_create_new_branch()}</SegmentedControlItem
      >
      <SegmentedControlItem value="existing"
        >{m.s8_use_existing_branch()}</SegmentedControlItem
      >
    </SegmentedControl>
  {/if}
  {#if loading}<p role="status" class="text-sm">
      {m.s8_reading_local_branches()}
    </p>{/if}
  {#if error}<p role="alert" class="break-all text-sm text-destructive">
      {error}
    </p>{/if}
  <WorktreeBranchFields
    {mode}
    bind:branch
    {startBranch}
    {disabled}
    {loading}
    hasData={!!data}
    failed={!!error}
    {refreshing}
    {baseSource}
    {upstream}
    {baseSubject}
    {lastRefreshedAt}
    {refreshWarning}
    localBranches={mappedLocalBranches}
    remoteBranches={mappedRemoteBranches}
    existingBranches={mappedExistingBranches}
    onchoosestartbranch={chooseStartBranch}
    onrefreshupstream={() => refreshUpstream(startBranch)}
    onsuggestbranch={() => (branch = suggestedBranch(branch || undefined))}
    onbranchedited={() => (branchEdited = true)}
  />
  <WorktreeAdvancedFields
    {mode}
    {deferred}
    bind:path
    bind:base
    {disabled}
    {loading}
    hasData={!!data}
    {commitRows}
    {selectedCommit}
    commitsCount={commits.length}
    {commitsLoading}
    {commitsError}
    onpathedited={() => (pathEdited = true)}
    onbaseedited={() => {
      baseEditVersion += 1;
      baseSource = "";
      baseSubject = "";
      upstream = "";
      lastRefreshedAt = null;
    }}
  />
</div>
