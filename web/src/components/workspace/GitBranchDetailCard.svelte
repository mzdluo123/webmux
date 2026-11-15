<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    resolveGitBranchActions,
    resolveGitBranchUpstream,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    activeBranch,
    context,
    branchFilter = null,
    onRouteOp,
  }: {
    activeBranch: GitBranchItem | null;
    context: GitWorktreeContext;
    branchFilter?: string | null;
    onRouteOp: (kind: GitSharedBranchActionKind, item: GitBranchItem) => void;
  } = $props();

  const activeUpstream = $derived(
    activeBranch ? resolveGitBranchUpstream(activeBranch) : null,
  );
  const activeActions = $derived(
    activeBranch
      ? resolveGitBranchActions({
          item: activeBranch,
          context,
          branchFilter,
          compareBaseOid: context.headOid ?? null,
          compareTargetLabel: m.s6_compare_with_head(),
        })
      : [],
  );
</script>

{#if !activeBranch}
  <p class="text-xs text-muted-foreground">
    {m.s6_branch_detail_empty()}
  </p>
{:else}
  {@const isCurrent =
    activeBranch.kind === "local" &&
    activeBranch.name === context.checkedOutBranch}
  {@const pushAct = activeActions.find((a) => a.kind === "push")}
  {@const ffAct = activeActions.find((a) => a.kind === "fast-forward")}
  {@const delAct = activeActions.find((a) => a.kind === "delete")}
  {@const renameAct = activeActions.find((a) => a.kind === "rename-branch")}
  {@const upstreamAct = activeActions.find((a) => a.kind === "set-upstream")}
  {@const createTagAct = activeActions.find((a) => a.kind === "create-tag")}
  {@const deleteTagAct = activeActions.find((a) => a.kind === "delete-tag")}
  <div class="space-y-3" data-testid="git-branch-detail-card">
    <div>
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <h3 class="font-mono text-sm font-semibold break-all text-foreground">
          {activeBranch.name}
        </h3>
        <Badge variant="outline">
          {#if activeBranch.kind === "tag"}
            {activeBranch.tagKind === "annotated"
              ? m.s6_tag_annotated()
              : m.s6_tag_lightweight()}
          {:else if activeBranch.kind === "local"}
            {m.s6_branch_kind_local()}
          {:else}
            {m.s6_branch_kind_remote_ref()}
          {/if}
        </Badge>
        {#if activeBranch.kind === "tag" && activeBranch.tagSigned}
          <Badge variant="info">{m.s6_tag_signed()}</Badge>
        {/if}
      </div>
      <p class="mt-1 text-xs wrap-anywhere text-muted-foreground">
        {m.s6_branch_points_to_commit({
          shortHead: activeBranch.shortHead,
          subject: activeBranch.subject,
        })}
      </p>
    </div>

    {#if activeBranch.kind === "tag"}
      <div class="space-y-1.5 rounded-md border bg-background p-2.5 text-xs">
        <p class="font-mono break-all text-muted-foreground">
          {m.s6_tag_target_commit_oid({ oid: activeBranch.head })}
        </p>
        {#if activeBranch.tagOid}
          <p class="font-mono break-all text-muted-foreground">
            {m.s6_tag_object_oid({ oid: activeBranch.tagOid })}
          </p>
        {/if}
        {#if activeBranch.taggerName}
          <p class="wrap-anywhere text-muted-foreground">
            {m.s6_tag_tagger_label({ name: activeBranch.taggerName })}
            {#if activeBranch.taggerEmail}
              &lt;{activeBranch.taggerEmail}&gt;
            {/if}
            {#if activeBranch.taggedAt}
              · {activeBranch.taggedAt}
            {/if}
          </p>
        {/if}
        {#if activeBranch.tagMessage}
          <p class="pt-1 whitespace-pre-wrap wrap-anywhere text-foreground">
            {activeBranch.tagMessage}
          </p>
        {/if}
      </div>
    {:else if activeBranch.kind === "local" && activeUpstream}
      <div class="rounded-md border bg-background p-2.5 text-xs">
        <p class="wrap-anywhere">{activeUpstream.detailText}</p>
      </div>
    {/if}

    {#if activeBranch.occupiedBy}
      <InlineNotice tone="warning">
        {m.s6_branch_occupied_notice_before()}
        <span class="font-mono break-all"
          >{activeBranch.occupiedBy.worktreePath}</span
        >
        {m.s6_branch_occupied_notice_after({
          count: activeBranch.occupiedBy.activeSessions,
        })}
      </InlineNotice>
    {/if}

    <div class="flex flex-wrap items-center gap-2 pt-1">
      <Button
        size="sm"
        variant="outline"
        onclick={() => onRouteOp("filter-log", activeBranch)}
      >
        {m.s6_branch_view_in_log()}
      </Button>

      {#if activeBranch.occupiedBy}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("open-occupied-worktree", activeBranch)}
        >
          {m.s6_branch_open_occupied_worktree()}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("open-new-worktree", activeBranch)}
        >
          {m.s6_branch_new_worktree_from_commit()}
        </Button>
      {:else if isCurrent && pushAct}
        <Button
          size="sm"
          variant="outline"
          disabled={pushAct.disabled}
          title={pushAct.disabledReason}
          onclick={() => onRouteOp("push", activeBranch)}
        >
          {pushAct.label}
        </Button>
      {:else if activeBranch.kind !== "tag" && context.worktreePath}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("checkout", activeBranch)}
        >
          {activeBranch.kind === "remote"
            ? m.s6_branch_checkout_and_track()
            : m.s6_branch_checkout_current_worktree()}
        </Button>
      {/if}

      {#if !activeBranch.occupiedBy && activeBranch.kind !== "tag"}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("create-branch-from", activeBranch)}
        >
          {m.s6_branch_create_from_here()}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("open-new-worktree", activeBranch)}
        >
          {m.s6_branch_open_in_new_worktree()}
        </Button>
      {/if}

      {#if activeBranch.kind === "local"}
        {#if renameAct}
          <Button
            size="sm"
            variant="outline"
            disabled={renameAct.disabled}
            title={renameAct.disabledReason}
            onclick={() => onRouteOp("rename-branch", activeBranch)}
            data-testid="git-branch-detail-rename-btn"
          >
            {m.s6_branch_rename_action()}
          </Button>
        {/if}
        {#if upstreamAct}
          <Button
            size="sm"
            variant="outline"
            onclick={() => onRouteOp("set-upstream", activeBranch)}
            data-testid="git-branch-detail-upstream-btn"
          >
            {m.s6_branch_set_upstream_action()}
          </Button>
        {/if}
        {#if ffAct}
          <Button
            size="sm"
            variant="outline"
            disabled={ffAct.disabled}
            title={ffAct.disabledReason}
            onclick={() => onRouteOp("fast-forward", activeBranch)}
          >
            {ffAct.label}
          </Button>
        {/if}
      {/if}

      {#if createTagAct}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onRouteOp("create-tag", activeBranch)}
          data-testid="git-branch-detail-create-tag-btn"
        >
          {m.s6_create_tag_ellipsis()}
        </Button>
      {/if}

      {#if activeBranch.kind === "tag" && deleteTagAct}
        <Button
          size="sm"
          variant="destructive"
          onclick={() => onRouteOp("delete-tag", activeBranch)}
          data-testid="git-branch-detail-delete-tag-btn"
        >
          {m.s6_branch_delete_local_tag()}
        </Button>
      {/if}

      {#if activeBranch.kind === "local" && delAct}
        <Button
          size="sm"
          variant="destructive"
          disabled={delAct.disabled}
          title={delAct.disabledReason}
          onclick={() => onRouteOp("delete", activeBranch)}
        >
          {delAct.label}
        </Button>
      {/if}
    </div>
  </div>
{/if}
