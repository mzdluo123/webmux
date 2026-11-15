<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconGitMerge from "~icons/tabler/git-merge";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitPrecedingCommit } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  export interface GitPushRecoveryCallbacks {
    onFetchRemote?: () => void;
    onFastForwardBranch?: (branch: string) => void;
    onMergeRemoteBranch?: (remoteRef: string) => void;
    onRebaseOntoRemoteBranch?: (remoteRef: string) => void;
  }

  let {
    precedingCommit = null,
    reason,
    sourceBranch,
    remoteTrackingRef,
    onCloseDialog,
    onFetchRemote,
    onFastForwardBranch,
    onMergeRemoteBranch,
    onRebaseOntoRemoteBranch,
  }: GitPushRecoveryCallbacks & {
    precedingCommit?: GitPrecedingCommit | null;
    reason: string;
    sourceBranch: string;
    remoteTrackingRef: string;
    onCloseDialog: () => void;
  } = $props();
</script>

<InlineNotice tone={precedingCommit ? "warning" : "danger"}>
  <div class="space-y-1.5">
    <p class={precedingCommit ? "font-medium" : ""}>
      {#if precedingCommit}
        {m.s6_push_rejected_with_commit({
          shortOid: precedingCommit.shortOid,
          subject: precedingCommit.subject,
          reason,
        })}
      {:else}
        {m.s6_push_rejected_plain({ reason })}
      {/if}
    </p>
    <div class="flex flex-wrap items-center gap-2">
      {#if onFetchRemote}
        <Button size="sm" variant="outline" wrap onclick={onFetchRemote}>
          <IconRefresh aria-hidden="true" />
          {m.s6_push_fetch_btn()}
        </Button>
      {/if}
      {#if onFastForwardBranch}
        <Button
          size="sm"
          variant="outline"
          wrap
          onclick={() => {
            const b = sourceBranch;
            onCloseDialog();
            onFastForwardBranch(b);
          }}
        >
          <IconArrowDown aria-hidden="true" />
          {m.s6_push_ff_update_btn()}
        </Button>
      {/if}
      {#if onMergeRemoteBranch && remoteTrackingRef}
        <Button
          size="sm"
          variant="outline"
          wrap
          onclick={() => {
            const r = remoteTrackingRef;
            onCloseDialog();
            onMergeRemoteBranch(r);
          }}
          data-testid="git-push-rejected-merge-btn"
        >
          <IconGitMerge aria-hidden="true" />
          {m.s6_push_merge_remote_btn()}
        </Button>
      {/if}
      {#if onRebaseOntoRemoteBranch && remoteTrackingRef}
        <Button
          size="sm"
          variant="default"
          wrap
          onclick={() => {
            const r = remoteTrackingRef;
            onCloseDialog();
            onRebaseOntoRemoteBranch(r);
          }}
          data-testid="git-push-rejected-rebase-btn"
        >
          <IconGitBranch aria-hidden="true" />
          {m.s6_push_rebase_remote_btn()}
        </Button>
      {/if}
    </div>
  </div>
</InlineNotice>
