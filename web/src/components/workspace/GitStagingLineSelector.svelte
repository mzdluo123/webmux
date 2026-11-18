<script lang="ts">
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import type {
    CodeDiffReaderState,
    GitLineSelection,
  } from "../../contracts/schemas";
  import {
    changedLineIndicesInHunk,
    countSelectedLines,
    isChangedHunkLine,
    selectAllChangedInHunk,
    toggleHunkLineSelection,
    toGitLineSelections,
    type HunkLineSelectionMap,
  } from "../../domains/workspace/git-line-selection";

  type GitChangeHunk = GitWorkingChange["hunks"][number];

  let {
    changeId,
    hunks,
    stageMode,
    canDiscardLines = false,
    busy = false,
    stale = false,
    onStageLines,
    onUnstageLines,
    onDiscardLines,
  }: {
    changeId: string;
    hunks: readonly GitChangeHunk[];
    stageMode: "stage" | "unstage" | "none";
    canDiscardLines?: boolean;
    busy?: boolean;
    stale?: boolean;
    onStageLines?: (
      changeId: string,
      selections: readonly GitLineSelection[],
    ) => void;
    onUnstageLines?: (
      changeId: string,
      selections: readonly GitLineSelection[],
    ) => void;
    onDiscardLines?: (
      changeId: string,
      selections: readonly GitLineSelection[],
    ) => void;
  } = $props();

  type LineAnchor = NonNullable<CodeDiffReaderState["top"]>;
  let viewport = $state<HTMLElement | null>(null);

  export function captureAnchor(): LineAnchor | undefined {
    if (!viewport) return;
    const edge = viewport.getBoundingClientRect().top;
    for (const row of viewport.querySelectorAll<HTMLElement>(
      "[data-line-old], [data-line-new]",
    )) {
      if (row.getBoundingClientRect().bottom <= edge + 0.5) continue;
      if (row.dataset.lineNew) {
        return { line: Number(row.dataset.lineNew), side: "additions" };
      }
      return { line: Number(row.dataset.lineOld), side: "deletions" };
    }
  }

  export function scrollToAnchor(anchor: LineAnchor) {
    if (!viewport) return;
    const key = anchor.side === "deletions" ? "lineOld" : "lineNew";
    const rows = Array.from(
      viewport.querySelectorAll<HTMLElement>(
        anchor.side === "deletions" ? "[data-line-old]" : "[data-line-new]",
      ),
    );
    const target =
      rows.find((row) => Number(row.dataset[key]) >= anchor.line) ??
      rows.at(-1);
    if (target) {
      viewport.scrollTop +=
        target.getBoundingClientRect().top -
        viewport.getBoundingClientRect().top;
    }
  }

  let selectionByChange = $state<{
    changeId: string;
    map: HunkLineSelectionMap;
    anchorByHunk: Record<string, number | null>;
  }>({ changeId: "", map: {}, anchorByHunk: {} });

  const selectionMap = $derived(
    selectionByChange.changeId === changeId ? selectionByChange.map : {},
  );
  const anchorByHunk = $derived(
    selectionByChange.changeId === changeId
      ? selectionByChange.anchorByHunk
      : {},
  );

  const selections = $derived(toGitLineSelections(selectionMap, hunks));
  const selectedCount = $derived(countSelectedLines(selections));

  function clearSelection() {
    selectionByChange = { changeId, map: {}, anchorByHunk: {} };
  }

  function handleLineToggle(
    hunk: GitChangeHunk,
    lineIndex: number,
    shiftKey: boolean,
  ) {
    const res = toggleHunkLineSelection({
      current: selectionMap,
      hunk,
      lineIndex,
      anchorIndex: anchorByHunk[hunk.id] ?? null,
      shiftKey,
    });
    selectionByChange = {
      changeId,
      map: res.next,
      anchorByHunk: { ...anchorByHunk, [hunk.id]: res.nextAnchor },
    };
  }

  function handleSelectAllHunk(hunk: GitChangeHunk) {
    selectionByChange = {
      changeId,
      map: selectAllChangedInHunk(selectionMap, hunk),
      anchorByHunk,
    };
  }

  function runStageSelected() {
    if (selections.length === 0) return;
    if (stageMode === "stage") {
      onStageLines?.(changeId, selections);
    } else if (stageMode === "unstage") {
      onUnstageLines?.(changeId, selections);
    }
    clearSelection();
  }

  function runDiscardSelected() {
    if (selections.length === 0 || !onDiscardLines) return;
    onDiscardLines(changeId, selections);
    clearSelection();
  }
</script>

<section
  class="flex min-h-44 min-w-0 flex-1 flex-col overflow-hidden bg-background"
  aria-label={m.s6_line_selector_aria()}
  data-testid="git-staging-line-selector"
>
  <div
    class="flex min-w-0 shrink-0 flex-wrap items-center justify-between gap-1.5 border-b bg-background px-2.5 py-1.5 text-xs"
    data-testid="git-line-selector-action-bar"
  >
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      <Badge variant={selectedCount > 0 ? "info" : "outline"}>
        {m.s6_line_selector_selected_count({ count: String(selectedCount) })}
      </Badge>
      <span class="text-muted-foreground">
        {m.s6_line_selector_hint()}
      </span>
    </div>

    <div class="flex min-w-0 flex-wrap items-center gap-1">
      {#if selectedCount > 0}
        <Button
          size="sm"
          variant="ghost"
          disabled={busy}
          onclick={clearSelection}
          data-testid="git-line-clear-btn"
        >
          <IconX aria-hidden="true" />
          {m.s6_line_selector_clear()}
        </Button>
      {/if}

      {#if canDiscardLines && onDiscardLines}
        <Button
          size="sm"
          variant="destructive-outline"
          wrap
          disabled={busy || stale || selectedCount === 0}
          onclick={runDiscardSelected}
          data-testid="git-line-discard-btn"
        >
          <IconTrash aria-hidden="true" />
          {m.s6_discard_selected_lines({ count: String(selectedCount) })}
        </Button>
      {/if}

      {#if stageMode === "stage"}
        <Button
          size="sm"
          variant="default"
          wrap
          disabled={busy || stale || selectedCount === 0}
          onclick={runStageSelected}
          data-testid="git-line-stage-btn"
        >
          <IconPlus aria-hidden="true" />
          {m.s6_stage_selected_lines({ count: String(selectedCount) })}
        </Button>
      {:else if stageMode === "unstage"}
        <Button
          size="sm"
          variant="outline"
          wrap
          disabled={busy || stale || selectedCount === 0}
          onclick={runStageSelected}
          data-testid="git-line-unstage-btn"
        >
          <IconMinus aria-hidden="true" />
          {m.s6_unstage_selected_lines({ count: String(selectedCount) })}
        </Button>
      {/if}
    </div>
  </div>

  <ScrollArea bind:viewportRef={viewport} class="min-h-8 flex-1">
    <div class="space-y-2 p-2">
      {#each hunks as hunk, hIdx (hunk.id)}
        {@const changedAll = changedLineIndicesInHunk(hunk)}
        {@const hunkSelected = selectionMap[hunk.id] ?? []}
        <div
          class="overflow-hidden rounded-md border bg-background"
          data-line-hunk-id={hunk.id}
        >
          <div
            class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 border-b px-2.5 py-1 text-xs"
          >
            <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
              <span class="shrink-0 font-mono text-muted-foreground">
                #{hIdx + 1}
              </span>
              <span
                class="min-w-0 flex-1 truncate font-mono text-muted-foreground"
                title={hunk.header}
              >
                {hunk.header}
              </span>
              {#if hunk.lines.some((l) => l.lineEnding === "crlf")}
                <Badge variant="outline">CRLF</Badge>
              {/if}
              {#if hunk.lines.some((l) => l.lineEnding === "none")}
                <Badge variant="outline">
                  {m.s6_line_selector_no_eof_newline()}
                </Badge>
              {/if}
            </div>
            {#if changedAll.length > 0}
              <Button
                size="sm"
                variant="ghost"
                disabled={busy || stale}
                onclick={() => handleSelectAllHunk(hunk)}
                data-testid={`git-line-select-hunk-${hIdx}`}
              >
                {hunkSelected.length === changedAll.length
                  ? m.s6_line_selector_deselect_hunk()
                  : m.s6_line_selector_select_hunk({
                      count: String(changedAll.length),
                    })}
              </Button>
            {/if}
          </div>

          <div
            class="divide-y divide-border/50 bg-background font-mono text-xs"
          >
            {#each hunk.lines as line, lIdx (`${hunk.id}:${lIdx}`)}
              {@const selectable = isChangedHunkLine(line)}
              {@const isChecked = hunkSelected.includes(lIdx)}
              {@const lineLabel =
                line.kind === "delete"
                  ? m.s6_line_kind_delete({
                      line: String(line.oldLine ?? "-"),
                    })
                  : line.kind === "add"
                    ? m.s6_line_kind_add({
                        line: String(line.newLine ?? "+"),
                      })
                    : m.s6_line_kind_context({
                        line: String(line.newLine ?? line.oldLine ?? ""),
                      })}
              {#if selectable}
                <div
                  class="flex min-w-0 items-start gap-2 px-2 py-1 hover:bg-muted/40"
                  data-line-kind={line.kind}
                  data-line-old={line.oldLine}
                  data-line-new={line.newLine}
                  data-line-selected={isChecked}
                  data-line-index={lIdx}
                >
                  <Checkbox
                    checked={isChecked}
                    disabled={busy || stale}
                    aria-label={m.s6_line_checkbox_aria({
                      hunk: String(hIdx + 1),
                      lineLabel,
                    })}
                    onclick={(e: MouseEvent) => {
                      e.preventDefault();
                      handleLineToggle(hunk, lIdx, e.shiftKey);
                    }}
                    class="mt-0.5 shrink-0"
                  />
                  <ListButton
                    variant="diff-line"
                    disabled={busy || stale}
                    onclick={(e) => handleLineToggle(hunk, lIdx, e.shiftKey)}
                  >
                    <span
                      class="w-14 shrink-0 text-right tabular-nums text-muted-foreground max-sm:w-10"
                    >
                      {line.oldLine ?? ""}
                      {line.newLine ?? ""}
                    </span>
                    <span
                      class={line.kind === "add"
                        ? "shrink-0 font-semibold text-success"
                        : "shrink-0 font-semibold text-destructive"}
                      aria-hidden="true"
                    >
                      {line.kind === "add" ? "+" : "−"}
                    </span>
                    <span
                      class={line.kind === "add"
                        ? "min-w-0 flex-1 whitespace-pre-wrap break-all text-success"
                        : "min-w-0 flex-1 whitespace-pre-wrap break-all text-destructive"}
                    >
                      {line.text || " "}
                    </span>
                  </ListButton>
                </div>
              {:else}
                <div
                  class="flex min-w-0 items-start gap-2 px-2 py-0.5 text-muted-foreground"
                  data-line-kind="context"
                  data-line-old={line.oldLine}
                  data-line-new={line.newLine}
                >
                  <span class="w-4 shrink-0" aria-hidden="true"></span>
                  <span
                    class="w-14 shrink-0 text-right tabular-nums max-sm:w-10"
                  >
                    {line.oldLine ?? ""}
                    {line.newLine ?? ""}
                  </span>
                  <span class="shrink-0" aria-hidden="true">&nbsp;</span>
                  <span class="min-w-0 flex-1 whitespace-pre-wrap break-all">
                    {line.text || " "}
                  </span>
                </div>
              {/if}
            {/each}
          </div>
        </div>
      {/each}
    </div>
  </ScrollArea>
</section>
