<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconGitMerge from "~icons/tabler/git-merge";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
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
        本地已提交（{precedingCommit.shortOid} · {precedingCommit.subject}），推送未完成：{reason}。本地提交已完整保留。
      {:else}
        推送未完成：{reason}
      {/if}
    </p>
    <div class="flex flex-wrap items-center gap-2">
      {#if onFetchRemote}
        <Button size="xs" variant="outline" wrap onclick={onFetchRemote}>
          <IconRefresh aria-hidden="true" />
          获取远程 (Fetch)
        </Button>
      {/if}
      {#if onFastForwardBranch}
        <Button
          size="xs"
          variant="outline"
          wrap
          onclick={() => {
            const b = sourceBranch;
            onCloseDialog();
            onFastForwardBranch(b);
          }}
        >
          <IconArrowDown aria-hidden="true" />
          快进更新… (Update)
        </Button>
      {/if}
      {#if onMergeRemoteBranch && remoteTrackingRef}
        <Button
          size="xs"
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
          合并远端分支… (Merge)
        </Button>
      {/if}
      {#if onRebaseOntoRemoteBranch && remoteTrackingRef}
        <Button
          size="xs"
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
          变基到远端分支… (Rebase)
        </Button>
      {/if}
    </div>
  </div>
</InlineNotice>
