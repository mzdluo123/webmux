<script lang="ts" module>
  import {
    previewSwitch,
    repositoryBranches,
    repositoryCommits,
    switchWorktree,
  } from "../../platform/api";
  import {
    gitSmartCheckout,
    gitSmartCheckoutPreview,
  } from "../../platform/git-v3-api";
  import type {
    Branches,
    Commit,
    GitSmartCheckoutPreview,
    GitSmartCheckoutPreviewRequest,
    GitSmartCheckoutRequest,
    GitSmartCheckoutResult,
    SwitchPreview,
    SwitchRequest,
    SwitchTarget,
    Worktree,
  } from "../../contracts/schemas";

  /** Where the switch reads and writes; stories pass fixtures. */
  export interface SwitchSource {
    branches(id: string, signal: AbortSignal): Promise<Branches>;
    commits(
      id: string,
      from: readonly string[],
      signal: AbortSignal,
    ): Promise<{ commits: Commit[] }>;
    preview(
      tree: Worktree,
      target: SwitchTarget,
      signal: AbortSignal,
    ): Promise<SwitchPreview>;
    switch(tree: Worktree, request: SwitchRequest): Promise<Worktree>;
    smartPreview?: (
      tree: Worktree,
      request: GitSmartCheckoutPreviewRequest,
      signal: AbortSignal,
    ) => Promise<GitSmartCheckoutPreview>;
    smartCheckout?: (
      tree: Worktree,
      request: GitSmartCheckoutRequest,
    ) => Promise<GitSmartCheckoutResult>;
  }
  export const apiSwitchSource: SwitchSource = {
    branches: repositoryBranches,
    commits: repositoryCommits,
    preview: previewSwitch,
    switch: switchWorktree,
    smartPreview: (tree, request, signal) =>
      gitSmartCheckoutPreview(tree.repositoryId, tree.id, request, signal),
    smartCheckout: (tree, request) =>
      gitSmartCheckout(tree.repositoryId, tree.id, request),
  };
</script>

<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { APIError } from "../../platform/api";
  import { commitChoices } from "../../domains/workspace/commit-choices";
  import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
  import { errorMessage } from "../../platform/error";
  import { LatestRequest } from "../../hooks/shared/remote-request.svelte";
  import { createSmartCheckoutController } from "../../hooks/workspace/git-smart-checkout.svelte";
  import type { WorktreePreset } from "../../domains/sessions/worktree-preset";
  import type { Repository } from "../../contracts/schemas";
  import {
    buildSwitchTarget,
    canConfirmSwitch,
    deriveSwitchBranchItems,
    resolveSwitchNewWorktreePreset,
    resolveSwitchOccupantPath,
    suggestSwitchBranchName,
  } from "../../domains/workspace/worktree-sync";
  import SwitchBranchForm from "./SwitchBranchForm.svelte";
  import SwitchBranchPreview from "./SwitchBranchPreview.svelte";
  import SwitchBranchSmartCheckout from "./SwitchBranchSmartCheckout.svelte";
  import SwitchBranchTargetFields, {
    type SwitchTargetMode,
  } from "./SwitchBranchTargetFields.svelte";

  // Switching a worktree's branch in place: choose an existing branch, a new
  // one or a commit to detach to; every choice is previewed by the server
  // (blockers, warnings, the sessions that keep running) and only a clean
  // preview can be confirmed (or carried across via tracked-stash Smart Checkout).
  let {
    formId,
    repository,
    worktree,
    initialBranch,
    initialMode = "existing",
    initialStartPoint,
    source = apiSwitchSource,
    busy = $bindable(false),
    ondone,
    onchange,
    onnewworktree,
    ongoto,
    onopenconflict,
    oncancel,
  }: {
    formId?: string;
    repository: Repository;
    worktree: Worktree;
    initialBranch?: string;
    initialMode?: "existing" | "new";
    initialStartPoint?: string;
    source?: SwitchSource;
    /** The switch runs; the host blocks closing and going back. */
    busy?: boolean;
    /** The switch finished; the worktree as it is now. */
    ondone: (tree: Worktree) => void;
    /** A switch ran, whatever its outcome; the repositories are read again. */
    onchange: () => void;
    /** Opens the chosen target in a new worktree instead. */
    onnewworktree: (preset: WorktreePreset) => void;
    /** Shows the worktree at a path: the one that has the target checked out. */
    ongoto: (path: string) => void;
    /** Opens the 3-way conflict resolver when Smart Checkout encounters conflicts. */
    onopenconflict?: (tree: Worktree, path?: string) => void;
    oncancel: () => void;
  } = $props();

  let mode = $state<SwitchTargetMode>(untrack(() => initialMode));

  let data = $state<Branches | null>(null);
  let loadError = $state("");
  const branchRequests = new LatestRequest();
  async function readBranches() {
    const request = branchRequests.begin();
    loadError = "";
    try {
      const result = await source.branches(repository.id, request.signal);
      if (!request.current()) return;
      data = result;
      if (initialBranch && !chosen) {
        const isLocal = result.branches.some((b) => b.name === initialBranch);
        chosen = { kind: isLocal ? "local" : "remote", name: initialBranch };
      }
      if (mode === "new" && !newName) newName = suggestion();
    } catch (err) {
      if (request.reportable()) loadError = errorMessage(err);
    }
  }

  const branchItems = $derived(
    deriveSwitchBranchItems(data, worktree.path, worktree.branch),
  );
  const localNames = $derived(branchItems.localNames);
  const remotes = $derived(branchItems.remotes);
  const localItems = $derived(branchItems.localItems);
  const remoteItems = $derived(branchItems.remoteItems);

  const localBranchList = $derived(
    (data?.branches ?? []).map((item) => ({ name: item.name })),
  );
  const remoteBranchList = $derived(
    remotes.map((item) => ({ name: item.name })),
  );
  let chosen = $state<{ kind: "local" | "remote"; name: string } | null>(null);
  let newName = $state("");
  let startPoint = $state(
    untrack(() => initialStartPoint ?? worktree.branch ?? worktree.head ?? ""),
  );
  const suggestion = (avoid?: string) =>
    suggestSwitchBranchName(localNames, repository, avoid);
  const nameTaken = $derived(localNames.has(newName.trim()));
  const startIsBranch = $derived(
    localNames.has(startPoint) || remotes.some((r) => r.name === startPoint),
  );

  let commits = $state<Commit[]>([]);
  let commitsRead = $state(false);
  let commitsError = $state("");
  let commit = $state("");
  const commitRequests = new LatestRequest();
  async function readCommits() {
    const request = commitRequests.begin();
    commitsError = "";
    try {
      const result = worktree.head
        ? await source.commits(repository.id, [worktree.head], request.signal)
        : { commits: [] };
      if (request.current()) commits = result.commits;
    } catch (err) {
      if (request.reportable()) commitsError = errorMessage(err);
    } finally {
      if (request.current()) commitsRead = true;
    }
  }
  const commitRows = $derived(
    commitChoices(commits, {
      tips: worktree.head
        ? [{ hash: worktree.head, tag: m.s8_tag_current() }]
        : [],
      base: "",
      baseSubject: "",
    }),
  );

  function changeMode(next: string) {
    if (next !== "existing" && next !== "new" && next !== "detach") return;
    if (next === mode) return;
    mode = next;
    if (next === "new" && !newName && data) newName = suggestion();
    if (next === "detach" && !commitsRead) void readCommits();
  }

  const target = $derived(
    buildSwitchTarget({
      mode,
      chosen,
      newName,
      nameTaken,
      startPoint,
      commit,
    }),
  );
  const targetKey = $derived(target ? JSON.stringify(target) : "");
  let preview = $state<SwitchPreview | null>(null);
  let previewing = $state(false);
  let previewError = $state("");
  let acknowledged = $state(false);
  let conflict = $state("");
  let error = $state("");
  const previews = new LatestRequest();

  const smart = createSmartCheckoutController({
    getWorktree: () => untrack(() => worktree),
    getTarget: () => target,
    getPlainPreview: () => preview,
    getBusy: () => busy,
    setBusy: (next) => {
      busy = next;
    },
    getSource: () => source,
    onDone: (tree) => ondone(tree),
    onChange: () => onchange(),
    onReloadPlainPreview: (next) => void readPreview(next),
  });

  onDestroy(() => {
    previews.dispose();
    branchRequests.dispose();
    commitRequests.dispose();
    smart.dispose();
  });
  void readBranches();

  async function readPreview(next: SwitchTarget) {
    const request = previews.begin();
    previewing = true;
    previewError = "";
    if (previewViewport) {
      previewViewport.scrollTop = 0;
      const outer = previewViewport.closest<HTMLElement>(
        '[data-slot="scroll-area-viewport"]',
      );
      if (outer && outer.scrollTop > previewViewport.offsetTop) {
        outer.scrollTop = previewViewport.offsetTop;
      }
    }
    try {
      const result = await source.preview(worktree, next, request.signal);
      if (!request.current()) return;
      preview = result;
      acknowledged = false;
      smart.syncPlainPreview(result);
    } catch (err) {
      if (request.reportable()) {
        preview = null;
        previewError = errorMessage(err);
        smart.syncPlainPreview(null);
      }
    } finally {
      if (request.current()) previewing = false;
    }
  }

  function resetPreview() {
    previews.cancel();
    preview = null;
    previewing = false;
    previewError = "";
    smart.resetAll();
  }

  $effect(() => {
    if (!targetKey) {
      untrack(resetPreview);
      return;
    }
    const next = untrack(() => target);
    if (!next) return;
    untrack(() => {
      previews.cancel();
      preview = null;
      conflict = "";
      error = "";
      previewing = true;
      smart.resetAll();
    });
    const timer = setTimeout(
      () => void readPreview(next),
      next.kind === "new" ? 300 : 0,
    );
    return () => clearTimeout(timer);
  });

  const occupant = $derived(resolveSwitchOccupantPath(preview));
  const canSwitch = $derived(
    canConfirmSwitch(preview, previewing, busy, acknowledged),
  );
  const newWorktreePreset = $derived(
    resolveSwitchNewWorktreePreset(preview, target, startIsBranch),
  );

  export function isSwitchReady() {
    return canSwitch;
  }

  let previewViewport = $state<HTMLElement | null>(null);

  function retryPreview() {
    if (!target || previewing || busy) return;
    previewViewport?.focus({ preventScroll: true });
    void readPreview(target);
  }

  async function submit() {
    if (!canSwitch || !preview || !target) return;
    const confirmed = preview;
    const again = target;
    previews.cancel();
    busy = true;
    error = "";
    conflict = "";
    try {
      const tree = await source.switch(worktree, {
        confirmationId: confirmed.confirmationId,
        clientRequestId: generateGitClientRequestId("switch"),
        acknowledgeSessions: acknowledged,
      });
      onchange();
      ondone(tree);
      return;
    } catch (err) {
      if (err instanceof APIError && err.status === 409)
        conflict = errorMessage(err);
      else error = errorMessage(err);
      onchange();
    } finally {
      busy = false;
    }
    preview = null;
    acknowledged = false;
    void readPreview(again);
  }
</script>

<SwitchBranchForm
  {formId}
  worktreePath={worktree.path}
  {busy}
  {canSwitch}
  bind:previewViewport
  onsubmit={() => void submit()}
  {oncancel}
>
  {#snippet fields()}
    <SwitchBranchTargetFields
      {mode}
      {busy}
      {data}
      {loadError}
      chosenName={chosen?.name ?? ""}
      {localItems}
      {remoteItems}
      {localBranchList}
      {remoteBranchList}
      bind:newName
      {nameTaken}
      bind:startPoint
      {startIsBranch}
      bind:commit
      {commitRows}
      {commitsRead}
      {commitsError}
      onModeChange={changeMode}
      onSelectBranch={(name) => {
        chosen = {
          kind: localNames.has(name) ? "local" : "remote",
          name,
        };
      }}
      onSuggestName={() => {
        newName = suggestion(newName || undefined);
      }}
    />
  {/snippet}

  {#snippet previewBody()}
    {#if busy}<p role="status" class="text-sm">
        {m.s8_preview_form_busy()}
      </p>{/if}
    {#if error}<p role="alert" class="text-sm break-all text-destructive">
        {error}
      </p>{/if}
    <SwitchBranchPreview
      {preview}
      {previewing}
      {previewError}
      {conflict}
      {worktree}
      bind:acknowledged
      {busy}
      onRetry={retryPreview}
    />

    <SwitchBranchSmartCheckout {smart} {worktree} {busy} {onopenconflict} />

    {#if occupant || newWorktreePreset}
      <div class="flex flex-wrap justify-end gap-2">
        {#if occupant}
          <Button
            type="button"
            variant="outline"
            wrap
            disabled={busy}
            title={occupant}
            onclick={() => ongoto(occupant)}>{m.s8_go_to_worktree()}</Button
          >
        {/if}
        {#if newWorktreePreset}
          <Button
            type="button"
            variant="success"
            wrap
            disabled={busy}
            title={m.s8_open_in_new_worktree_instead_title()}
            onclick={() => onnewworktree(newWorktreePreset)}
            >{m.s8_open_in_new_worktree_instead()}</Button
          >
        {/if}
      </div>
    {/if}
  {/snippet}
</SwitchBranchForm>
