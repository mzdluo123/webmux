<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type {
    GitCommitDetail,
    GitCommitFile,
  } from "../../contracts/git-workbench-schemas";
  import { resolveCommitParentFiles } from "../../domains/workspace/git-workbench-state";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import type { GitPushFileDiffState } from "../../hooks/workspace/git-push-controller.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    detail,
    detailLoading = false,
    selectedParentOverride = "",
    selectedFilePath = null,
    liveDiffState = null,
    onSelectParent,
    onSelectFile,
  }: {
    detail: GitCommitDetail | null;
    detailLoading?: boolean;
    selectedParentOverride?: string;
    selectedFilePath?: string | null;
    liveDiffState?: GitPushFileDiffState | null;
    onSelectParent?: (parentOid: string) => void;
    onSelectFile: (path: string) => void;
  } = $props();

  const offline = createOfflineHunkDiffState();

  const resolved = $derived(
    detail
      ? resolveCommitParentFiles(detail, selectedParentOverride || undefined)
      : { selectedParent: "", files: [] as GitCommitFile[] },
  );
  const activeFile = $derived(
    resolved.files.find((f) => (f.newPath || f.oldPath) === selectedFilePath) ??
      resolved.files[0] ??
      null,
  );

  // Offline / Storybook fallback via createOfflineHunkDiffState
  $effect(() =>
    offline.load({
      disabled: Boolean(liveDiffState),
      hasTarget: Boolean(activeFile),
      binary: Boolean(activeFile?.binary),
      binaryReason: "二进制文件不直接展开文本行级差异。",
      emptyReason: "该文件仅包含模式或重命名变更，无文本差异。",
      snapshotKey: `push-fixture:${detail?.oid ?? ""}:${activeFile?.newPath ?? ""}`,
      path: activeFile?.newPath ?? "",
      oldPath: activeFile?.oldPath ?? "",
      status: activeFile?.status ?? "",
      hunks: activeFile?.hunks ?? [],
    }),
  );

  const diffPhase = $derived(
    liveDiffState ? liveDiffState.phase : offline.state.phase,
  );
  const diffMeta = $derived(
    liveDiffState ? liveDiffState.fileDiff : offline.state.fileDiff,
  );
  const diffReason = $derived(
    liveDiffState ? liveDiffState.reason : offline.state.reason,
  );
</script>

<div class="flex min-w-0 flex-col gap-2.5">
  <div class="rounded-lg border bg-background p-2">
    <div class="flex flex-wrap items-center justify-between gap-2 px-1 pb-1">
      <span class="text-xs font-semibold text-foreground">
        选中提交变更文件 ({resolved.files.length})
      </span>
      {#if detail && detail.parents.length > 1 && onSelectParent}
        <div class="flex items-center gap-1.5 text-xs">
          <span class="text-muted-foreground">合并父提交：</span>
          <NativeSelect.Root
            size="sm"
            value={resolved.selectedParent}
            aria-label="选择合并父提交"
            onchange={(e) => onSelectParent(e.currentTarget.value)}
          >
            {#each detail.parents as parent, idx (parent.oid)}
              <NativeSelect.Option value={parent.oid}>
                Parent #{idx + 1} ({parent.shortOid})
              </NativeSelect.Option>
            {/each}
          </NativeSelect.Root>
        </div>
      {/if}
    </div>

    {#if detailLoading}
      <p class="px-2 py-2 text-xs text-muted-foreground">
        正在读取该提交的变更文件列表…
      </p>
    {:else if !detail}
      <p class="px-2 py-2 text-xs text-muted-foreground">
        选择左侧任意待推送提交查看其变更文件与提交说明。
      </p>
    {:else if resolved.files.length === 0}
      <p class="px-2 py-2 text-xs text-muted-foreground">
        该提交不包含文本或路径变更。
      </p>
    {:else}
      <ScrollArea viewportClasses="max-h-32">
        <div class="space-y-0.5">
          {#each resolved.files as file (file.newPath || file.oldPath)}
            {@const fPath = file.newPath || file.oldPath}
            <ListButton
              variant="compact-row"
              aria-selected={(activeFile?.newPath || activeFile?.oldPath) ===
                fPath}
              onclick={() => onSelectFile(fPath)}
            >
              <Badge variant="outline">{file.status}</Badge>
              <span class="min-w-0 flex-1 truncate font-mono" title={fPath}>
                {file.oldPath && file.oldPath !== file.newPath
                  ? `${file.oldPath} → ${file.newPath}`
                  : fPath}
              </span>
            </ListButton>
          {/each}
        </div>
      </ScrollArea>
    {/if}
  </div>

  {#if activeFile}
    <div
      class="code-panel flex h-44 min-w-0 flex-col overflow-hidden rounded-lg border bg-background sm:h-52"
      data-testid="git-push-file-diff"
    >
      {#if diffPhase === "loading"}
        <p class="p-3 text-xs text-muted-foreground">
          正在加载文件差异（{activeFile.newPath || activeFile.oldPath}）…
        </p>
      {:else if diffPhase === "ready" && diffMeta}
        <CodeDiffReader
          fileDiff={diffMeta}
          side="new"
          expired={false}
          draftEnabled={false}
          preferredDiffStyle="unified"
          onselect={() => {}}
        />
      {:else}
        <div class="p-3">
          <InlineNotice tone={diffPhase === "error" ? "danger" : "info"}>
            {diffReason || "当前变更文件无文本差异预览。"}
          </InlineNotice>
        </div>
      {/if}
    </div>
  {/if}

  {#if detail}
    <div
      class="rounded-lg border bg-background p-2.5 text-xs"
      data-testid="git-push-commit-meta"
    >
      <div
        class="flex flex-wrap items-center justify-between gap-1.5 text-muted-foreground"
      >
        <span class="font-mono">{detail.shortOid}</span>
        <span>{detail.authorName} &lt;{detail.authorEmail}&gt;</span>
        <span>{detail.authorAt}</span>
      </div>
      <p class="mt-1 font-medium wrap-anywhere text-foreground">
        {detail.subject}
      </p>
      {#if detail.body}
        <p
          class="mt-1 font-mono whitespace-pre-wrap wrap-anywhere text-muted-foreground"
        >
          {detail.body}
        </p>
      {/if}
    </div>
  {/if}
</div>
