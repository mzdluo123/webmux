<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type {
    GitDiffLine,
    GitStagingHunk,
  } from "../../contracts/git-workbench-schemas";

  let {
    path,
    oldPath = "",
    status = "M",
    binary = false,
    degradeReason = "",
    hunks = [],
    stageMode = "none",
    hunkStageAvailable = false,
    onStageHunk,
    onUnstageHunk,
  }: {
    path: string;
    oldPath?: string;
    status?: string;
    binary?: boolean;
    degradeReason?: string;
    hunks?: readonly GitStagingHunk[];
    stageMode?: "none" | "stage" | "unstage";
    hunkStageAvailable?: boolean;
    onStageHunk?: (hunkId: string) => void;
    onUnstageHunk?: (hunkId: string) => void;
  } = $props();

  const wide = new MediaQuery("(min-width: 768px)");
  let viewMode = $state<"unified" | "split">("unified");
  const effectiveMode = $derived(wide.current ? viewMode : "unified");

  interface SplitPair {
    left: GitDiffLine | null;
    right: GitDiffLine | null;
  }

  function buildSplitPairs(lines: readonly GitDiffLine[]): SplitPair[] {
    const out: SplitPair[] = [];
    let i = 0;
    while (i < lines.length) {
      const cur = lines[i]!;
      if (cur.kind === "context") {
        out.push({ left: cur, right: cur });
        i++;
        continue;
      }
      const dels: GitDiffLine[] = [];
      const adds: GitDiffLine[] = [];
      while (i < lines.length && lines[i]?.kind === "delete") {
        dels.push(lines[i]!);
        i++;
      }
      while (i < lines.length && lines[i]?.kind === "add") {
        adds.push(lines[i]!);
        i++;
      }
      const maxLen = Math.max(dels.length, adds.length);
      for (let j = 0; j < maxLen; j++) {
        out.push({ left: dels[j] ?? null, right: adds[j] ?? null });
      }
    }
    return out;
  }

  function linePrefix(kind: GitDiffLine["kind"]): string {
    if (kind === "add") return "+";
    if (kind === "delete") return "-";
    return " ";
  }
</script>

<div
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface text-foreground"
  data-slot="git-diff-viewer"
>
  <div
    class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-3 py-1.5"
  >
    <div class="flex min-w-0 items-center gap-2">
      <Badge variant="outline">{status}</Badge>
      <span class="truncate font-mono text-xs" title={path}>
        {oldPath && oldPath !== path ? `${oldPath} → ${path}` : path}
      </span>
    </div>
    {#if wide.current && !binary && hunks.length > 0}
      <SegmentedControl.Root
        value={viewMode}
        onValueChange={(val) => {
          if (val === "unified" || val === "split") viewMode = val;
        }}
        aria-label="差异显示方式"
      >
        <SegmentedControl.Item value="unified">单栏</SegmentedControl.Item>
        <SegmentedControl.Item value="split">双栏对比</SegmentedControl.Item>
      </SegmentedControl.Root>
    {/if}
  </div>

  <div class="min-h-0 flex-1 overflow-y-auto p-3">
    {#if degradeReason}
      <p
        class="mb-2 rounded-md border border-warning/30 bg-warning/10 px-2.5 py-1.5 text-xs wrap-anywhere text-warning"
      >
        代码块级操作已降级为整文件处理：{degradeReason}
      </p>
    {/if}

    {#if binary}
      <p class="text-xs text-muted-foreground">
        二进制文件变更不展示文本差异，仅支持整文件暂存或对比。
      </p>
    {:else if hunks.length === 0}
      <p class="text-xs text-muted-foreground">
        当前条目没有可展开的文本差异块。
      </p>
    {:else}
      <div class="space-y-3">
        {#each hunks as hunk (hunk.id)}
          <section
            class="overflow-hidden rounded-md border border-git-border bg-git-surface"
            aria-label={`代码块 ${hunk.header}`}
          >
            <div
              class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-2.5 py-1"
            >
              <span
                class="min-w-0 max-w-full flex-1 truncate font-mono text-xs text-muted-foreground"
              >
                {hunk.header}
              </span>
              {#if stageMode === "stage" && hunkStageAvailable && onStageHunk}
                <Button
                  size="xs"
                  variant="outline"
                  onclick={() => onStageHunk(hunk.id)}
                >
                  暂存此块
                </Button>
              {:else if stageMode === "unstage" && hunkStageAvailable && onUnstageHunk}
                <Button
                  size="xs"
                  variant="outline"
                  onclick={() => onUnstageHunk(hunk.id)}
                >
                  取消暂存此块
                </Button>
              {/if}
            </div>

            {#if effectiveMode === "split"}
              <div class="divide-y divide-git-border font-mono text-xs">
                {#each buildSplitPairs(hunk.lines) as pair, idx (idx)}
                  <div
                    class="grid min-w-0 grid-cols-2 divide-x divide-git-border"
                  >
                    <div
                      class={pair.left?.kind === "delete"
                        ? "flex min-w-0 items-start gap-1.5 bg-destructive/10 px-2 py-0.5 text-foreground"
                        : "flex min-w-0 items-start gap-1.5 px-2 py-0.5 text-foreground"}
                    >
                      <span
                        class="w-7 shrink-0 text-right text-muted-foreground select-none"
                      >
                        {pair.left?.oldLine ?? ""}
                      </span>
                      <span class="w-3 shrink-0 select-none">
                        {pair.left ? linePrefix(pair.left.kind) : " "}
                      </span>
                      <span
                        class="min-w-0 flex-1 break-all whitespace-pre-wrap"
                      >
                        {pair.left?.text ?? ""}
                      </span>
                    </div>
                    <div
                      class={pair.right?.kind === "add"
                        ? "flex min-w-0 items-start gap-1.5 bg-success/10 px-2 py-0.5 text-foreground"
                        : "flex min-w-0 items-start gap-1.5 px-2 py-0.5 text-foreground"}
                    >
                      <span
                        class="w-7 shrink-0 text-right text-muted-foreground select-none"
                      >
                        {pair.right?.newLine ?? ""}
                      </span>
                      <span class="w-3 shrink-0 select-none">
                        {pair.right ? linePrefix(pair.right.kind) : " "}
                      </span>
                      <span
                        class="min-w-0 flex-1 break-all whitespace-pre-wrap"
                      >
                        {pair.right?.text ?? ""}
                      </span>
                    </div>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="divide-y divide-git-border/60 font-mono text-xs">
                {#each hunk.lines as line, lineIdx (lineIdx)}
                  <div
                    class={line.kind === "add"
                      ? "flex min-w-0 flex-wrap items-start gap-1.5 bg-success/10 px-2 py-0.5"
                      : line.kind === "delete"
                        ? "flex min-w-0 flex-wrap items-start gap-1.5 bg-destructive/10 px-2 py-0.5"
                        : "flex min-w-0 flex-wrap items-start gap-1.5 px-2 py-0.5"}
                  >
                    <span
                      class="w-7 shrink-0 text-right text-muted-foreground select-none"
                    >
                      {line.oldLine ?? ""}
                    </span>
                    <span
                      class="w-7 shrink-0 text-right text-muted-foreground select-none"
                    >
                      {line.newLine ?? ""}
                    </span>
                    <span class="w-3 shrink-0 font-semibold select-none">
                      {linePrefix(line.kind)}
                    </span>
                    <span
                      class="min-w-[8ch] max-w-full flex-1 break-all whitespace-pre-wrap"
                    >
                      {line.text}
                    </span>
                    {#if line.lineEnding === "crlf"}
                      <Badge variant="outline">CRLF</Badge>
                    {:else if line.lineEnding === "none"}
                      <Badge variant="outline" class="max-w-full">
                        <span class="truncate">无末尾换行</span>
                      </Badge>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </section>
        {/each}
      </div>
    {/if}
  </div>
</div>
