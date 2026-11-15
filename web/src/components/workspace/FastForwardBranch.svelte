<script lang="ts">
  import { onMount } from "svelte";
  import * as m from "$lib/paraglide/messages.js";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import {
    branchOperations,
    type BranchFlowProps,
  } from "../../domains/workspace/branch-operations";
  import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
  import { PreviewFlow } from "../../hooks/workspace/preview-flow.svelte";
  import type { FastForwardPreview } from "../../contracts/schemas";
  import BranchCommitList from "./BranchCommitList.svelte";
  import FlowActions from "../shared/FlowActions.svelte";
  import FlowStatus from "../shared/FlowStatus.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import PreviewIssues from "./PreviewIssues.svelte";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";

  // Fast-forwards a local branch to its upstream's tip, from the
  // remote-tracking refs at hand: in the worktree that has it checked out,
  // or by moving only the branch ref. Running sessions there keep running
  // and must be acknowledged. Held by a dialog or a repository panel
  // subview, which `busy` keeps open while the request runs.
  let {
    repositoryId,
    branch,
    busy = $bindable(false),
    source = branchOperations,
    onchange,
    onclose,
  }: BranchFlowProps & { branch: string } = $props();

  const flow = new PreviewFlow<FastForwardPreview>((signal) =>
    source.fastForwardPreview(repositoryId, branch, signal),
  );
  onMount(() => {
    void flow.refresh();
    return () => flow.dispose();
  });
  const preview = $derived(flow.preview);
  // The acknowledgement covers the sessions of the preview it was given
  // for; a new preview asks again.
  let acknowledgedFor = $state.raw<FastForwardPreview | null>(null);
  const acknowledged = $derived(!!preview && acknowledgedFor === preview);
  let done = $state<FastForwardPreview | null>(null);
  const sessions = $derived(preview?.sessions ?? []);
  const ready = $derived(
    !!preview?.confirmationId &&
      !flow.loading &&
      !flow.running &&
      (sessions.length === 0 || acknowledged),
  );

  async function confirm() {
    const target = preview;
    if (!target || !ready) return;
    busy = true;
    const result = await flow.run(() =>
      source.fastForward(
        repositoryId,
        target.confirmationId,
        sessions.length > 0 && acknowledged,
        {
          clientRequestId: generateGitClientRequestId("fast_forward"),
          worktreeId: target.worktree?.id,
        },
      ),
    );
    busy = false;
    onchange();
    if (result) done = target;
  }
  const short = (hash: string) => hash.slice(0, 8);
</script>

<div class="min-w-0 space-y-3">
  {#if done}
    <InlineNotice tone="success"
      >{m.s8_ff_done_before()}<span class="font-mono break-all"
        >{done.branch}</span
      >{m.s8_ff_done_mid()}<span class="font-mono break-all"
        >{done.upstream}</span
      >{m.s8_ff_done_after({ commit: short(done.to) })}</InlineNotice
    >
  {:else}
    <FlowStatus {flow} />
    {#if preview}
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
        <dt class="text-muted-foreground">{m.s8_label_branch()}</dt>
        <dd class="font-mono break-all">{preview.branch}</dd>
        {#if preview.upstream}
          <dt class="text-muted-foreground">{m.s8_label_upstream()}</dt>
          <dd class="font-mono break-all">{preview.upstream}</dd>
        {/if}
        {#if preview.to}
          <dt class="text-muted-foreground">{m.s8_label_commit()}</dt>
          <dd class="font-mono">{short(preview.from)} → {short(preview.to)}</dd>
        {/if}
        <dt class="text-muted-foreground">{m.s8_label_worktree()}</dt>
        <dd class="break-all">
          {#if preview.worktree}<span class="font-mono"
              >{preview.worktree.path}</span
            >{:else}{m.s8_ff_not_checked_out()}{/if}
        </dd>
      </dl>
      <PreviewIssues blockers={preview.blockers} warnings={preview.warnings} />
      {#if preview.commits.total > 0}
        <BranchCommitList
          title={m.s8_ff_commits_title()}
          commits={preview.commits}
        />
      {/if}
      {#if sessions.length}
        <div class="space-y-2 rounded-lg border p-3">
          <WorktreeSessionList
            title={m.s8_ff_running_sessions_title()}
            {sessions}
          />
          <Label class="items-start">
            <Checkbox
              checked={acknowledged}
              onCheckedChange={(value) =>
                (acknowledgedFor = value ? preview : null)}
              disabled={busy}
              class="mt-0.5"
            />
            <span class="text-sm leading-snug"
              >{m.s8_ff_ack_sessions({ count: String(sessions.length) })}</span
            >
          </Label>
        </div>
      {/if}
    {/if}
  {/if}
  <FlowActions
    done={!!done}
    {busy}
    {ready}
    label={busy ? m.s8_ff_busy() : m.s8_branch_action_fast_forward()}
    onconfirm={() => void confirm()}
    {onclose}
  />
</div>
