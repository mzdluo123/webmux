<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { resolveCommitParentFiles } from "../../domains/workspace/git-workbench-state";
  import type { GitPushPreviewModel } from "../../contracts/git-workbench-schemas";
  import GitDiffViewer from "./GitDiffViewer.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  let {
    model,
    onConfirm,
    onClose,
  }: {
    model: GitPushPreviewModel;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  let selectedCommitOid = $state<string | null>(null);
  let selectedFilePath = $state<string | null>(null);

  const activeCommit = $derived(
    model.commits.find((c) => c.oid === selectedCommitOid) ??
      model.commits[0] ??
      null,
  );
  const resolved = $derived(
    activeCommit
      ? resolveCommitParentFiles(activeCommit)
      : { selectedParent: "", files: [] },
  );
  const activeFile = $derived(
    resolved.files.find((f) => f.newPath === selectedFilePath) ??
      resolved.files[0] ??
      null,
  );
  const canPush = $derived(
    model.blockers.length === 0 && model.commits.length > 0,
  );
</script>

<ResponsiveDialog
  title={`推送确认 · ${model.sourceBranch} → ${model.remote}/${model.targetBranch}`}
  description={`即将向脱敏远端 ${model.pushUrl} 推送 ${model.commits.length} 个提交。`}
  drawerDescription={`向 ${model.remote}/${model.targetBranch} 推送 ${model.commits.length} 个提交。`}
  truncateDrawerTitle
  class="sm:max-w-4xl"
  onclose={onClose}
>
  <div class="space-y-4">
    <div
      class="flex min-w-0 flex-wrap items-center justify-between gap-2 rounded-lg border border-git-border bg-git-panel p-2.5 text-xs"
    >
      <div class="flex min-w-0 max-w-full flex-wrap items-center gap-1.5">
        <Badge variant="info" class="max-w-full">
          <span class="truncate" title={model.sourceBranch}>
            {model.sourceBranch}
          </span>
        </Badge>
        <span class="text-muted-foreground">→</span>
        <Badge variant="outline" class="max-w-full">
          <span
            class="truncate"
            title={`${model.remote}/${model.targetBranch}`}
          >
            {model.remote}/{model.targetBranch}
          </span>
        </Badge>
        {#if model.publish}
          <Badge variant="warning" class="max-w-full">
            <span class="truncate">首次发布上游 (Publish)</span>
          </Badge>
        {/if}
      </div>
      <span class="font-mono break-all text-muted-foreground">
        {model.pushUrl}
      </span>
    </div>

    <GitIssueList blockers={model.blockers} warnings={model.warnings} />

    <div
      class="grid min-w-0 grid-cols-1 gap-3 lg:grid-cols-12"
      data-slot="git-push-body"
    >
      <div
        class="min-w-0 space-y-2 rounded-lg border border-git-border bg-git-surface p-2 lg:col-span-5"
      >
        <span class="px-1 text-xs font-semibold text-muted-foreground">
          待推送提交 ({model.commits.length})
        </span>
        {#if model.commits.length === 0}
          <p class="px-2 py-3 text-xs text-muted-foreground">
            当前分支没有领先远端的待推提交。
          </p>
        {:else}
          <div class="max-h-56 space-y-0.5 overflow-y-auto">
            {#each model.commits as commit (commit.oid)}
              <ListButton
                variant="git-row"
                aria-selected={activeCommit?.oid === commit.oid}
                onclick={() => {
                  selectedCommitOid = commit.oid;
                  selectedFilePath = null;
                }}
              >
                <span class="shrink-0 font-mono text-xs text-muted-foreground">
                  {commit.shortOid}
                </span>
                <span
                  class="min-w-0 flex-1 truncate font-medium"
                  title={commit.subject}
                >
                  {commit.subject}
                </span>
              </ListButton>
            {/each}
          </div>
        {/if}

        {#if activeCommit}
          <div class="border-t border-git-border pt-2">
            <span class="px-1 text-xs font-semibold text-muted-foreground">
              变更文件 ({resolved.files.length})
            </span>
            <div class="mt-1 max-h-36 space-y-0.5 overflow-y-auto">
              {#each resolved.files as file (file.newPath)}
                <ListButton
                  variant="git-row"
                  aria-selected={activeFile?.newPath === file.newPath}
                  onclick={() => (selectedFilePath = file.newPath)}
                >
                  <Badge variant="outline">{file.status}</Badge>
                  <span
                    class="min-w-0 flex-1 truncate font-mono"
                    title={file.newPath}
                  >
                    {file.newPath}
                  </span>
                </ListButton>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <div
        class="min-h-60 min-w-0 overflow-hidden rounded-lg border border-git-border bg-git-surface lg:col-span-7"
      >
        {#if activeFile}
          <GitDiffViewer
            path={activeFile.newPath}
            oldPath={activeFile.oldPath}
            status={activeFile.status}
            binary={activeFile.binary}
            hunks={activeFile.hunks}
          />
        {:else}
          <p class="p-4 text-xs text-muted-foreground">
            选择左侧待推提交与文件核对差异。
          </p>
        {/if}
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-end gap-2 pt-1">
      <Button variant="outline" onclick={onClose}>取消</Button>
      <Button variant="git-primary" disabled={!canPush} onclick={onConfirm}>
        {model.publish ? "发布并推送" : "确认推送"}
      </Button>
    </div>
  </div>
</ResponsiveDialog>
