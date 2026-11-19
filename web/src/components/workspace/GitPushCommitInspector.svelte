<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitCommitDetail,
    GitCommitFile,
  } from "../../contracts/git-workbench-schemas";
  import { resolveCommitParentFiles } from "../../domains/workspace/git-workbench-state";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import type { GitPushFileDiffState } from "../../hooks/workspace/git-push-controller.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

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
    detailLoading
      ? null
      : (resolved.files.find(
          (f) => (f.newPath || f.oldPath) === selectedFilePath,
        ) ??
          resolved.files[0] ??
          null),
  );

  const activePath = $derived(activeFile?.newPath || activeFile?.oldPath || "");
  const fileListKey = $derived(
    JSON.stringify([detail?.oid, resolved.selectedParent]),
  );
  const diffKey = $derived(JSON.stringify([fileListKey, activePath]));

  // Offline / Storybook fallback via createOfflineHunkDiffState
  $effect(() =>
    offline.load({
      disabled: Boolean(liveDiffState),
      hasTarget: Boolean(activeFile),
      binary: Boolean(activeFile?.binary),
      binaryReason: m.s6_binary_no_text_diff(),
      emptyReason: m.s6_push_inspector_mode_rename_only(),
      snapshotKey: diffKey,
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
    <div
      class="flex min-h-9 flex-wrap items-center justify-between gap-2 px-1 pb-1"
    >
      <span class="text-xs font-semibold text-foreground">
        {m.s6_push_inspector_files_heading({
          count: String(resolved.files.length),
        })}
      </span>
      {#if detail && detail.parents.length > 1 && onSelectParent}
        <div class="flex min-w-0 items-center gap-1.5 text-xs">
          <span class="text-muted-foreground"
            >{m.s6_push_inspector_merge_parent()}</span
          >
          <NativeSelect.Root
            size="sm"
            value={resolved.selectedParent}
            aria-label={m.s6_push_inspector_select_parent_aria()}
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

    <div class="min-w-0">
      {#if detailLoading}
        <p class="px-2 py-2 text-xs text-muted-foreground">
          {m.s6_push_inspector_loading_files()}
        </p>
      {:else if !detail}
        <p class="px-2 py-2 text-xs text-muted-foreground">
          {m.s6_push_inspector_select_prompt()}
        </p>
      {:else if resolved.files.length === 0}
        <p class="px-2 py-2 text-xs text-muted-foreground">
          {m.s6_push_inspector_no_changes()}
        </p>
      {:else}
        {#key fileListKey}
          <ScrollArea
            class="max-h-32"
            viewportClasses="max-h-32 overscroll-contain"
          >
            <div class="space-y-0.5">
              {#each resolved.files as file (file.newPath || file.oldPath)}
                {@const fPath = file.newPath || file.oldPath}
                <ListButton
                  variant="compact-row"
                  aria-selected={(activeFile?.newPath ||
                    activeFile?.oldPath) === fPath}
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
        {/key}
      {/if}
    </div>
  </div>

  {#if activeFile || detailLoading}
    <div
      class="code-panel flex h-44 min-w-0 flex-col overflow-hidden rounded-lg border bg-background sm:h-52"
      data-testid="git-push-file-diff"
    >
      {#if activePath}
        <p
          class="shrink-0 truncate border-b px-2 py-1 font-mono text-xs text-muted-foreground"
          title={activePath}
        >
          {activePath}
        </p>
      {/if}
      {#if !detailLoading}
        {#if diffPhase === "loading" || (liveDiffState && liveDiffState.path !== activePath)}
          <p class="p-3 text-xs text-muted-foreground" role="status">
            {m.s6_loading_ellipsis()}
          </p>
        {:else if diffPhase === "ready" && diffMeta}
          {#key diffKey}
            <CodeDiffReader
              fileDiff={diffMeta}
              side={(liveDiffState?.status || activeFile?.status) === "D" ||
              (diffMeta.additionLines.length === 0 &&
                diffMeta.deletionLines.length > 0)
                ? "old"
                : "new"}
              expired={false}
              draftEnabled={false}
              preferredDiffStyle="split"
              onselect={() => {}}
            />
          {/key}
        {:else}
          <div class="p-3">
            <InlineNotice tone={diffPhase === "error" ? "danger" : "info"}>
              {diffReason || m.s6_push_inspector_no_diff_preview()}
            </InlineNotice>
          </div>
        {/if}
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
        <span class="min-w-0 wrap-anywhere"
          >{detail.authorName} &lt;{detail.authorEmail}&gt;</span
        >
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
