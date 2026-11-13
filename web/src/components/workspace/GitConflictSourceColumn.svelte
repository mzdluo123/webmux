<script lang="ts">
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconArrowRight from "~icons/tabler/arrow-right";
  import IconChevronsLeft from "~icons/tabler/chevrons-left";
  import IconChevronsRight from "~icons/tabler/chevrons-right";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitConflictSegment,
    GitConflictSide,
  } from "../../contracts/schemas";
  import type {
    GitConflictChunkChoice,
    GitConflictDraftState,
  } from "../../domains/workspace/git-conflict-model";

  let {
    side,
    info,
    segments,
    draft,
    showBase = false,
    activeConflictSegmentId = null,
    onChooseChunk,
    onToggleNonConflicting,
  }: {
    side: "left" | "right";
    info: GitConflictSide;
    segments: readonly GitConflictSegment[];
    draft: GitConflictDraftState;
    showBase?: boolean;
    activeConflictSegmentId?: string | null;
    onChooseChunk: (segmentId: string, choice: GitConflictChunkChoice) => void;
    onToggleNonConflicting: (segmentId: string) => void;
  } = $props();

  const isLeft = $derived(side === "left");
  const short = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const primaryChoice = $derived<GitConflictChunkChoice>(
    isLeft ? "left" : "right",
  );
  const bothChoice = $derived<GitConflictChunkChoice>(
    isLeft ? "both_lr" : "both_rl",
  );
  const changeKind = $derived(isLeft ? "change_left" : "change_right");
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label={isLeft ? "左侧只读来源版本" : "右侧只读来源版本"}
  data-testid={isLeft
    ? "git-conflict-left-column"
    : "git-conflict-right-column"}
>
  <div class="space-y-1 border-b bg-background px-2.5 py-2 text-xs">
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      <Badge variant={isLeft ? "info" : "outline"}>
        {isLeft ? "左侧来源" : "右侧来源"}
      </Badge>
      <span class="min-w-0 truncate font-semibold" title={info.label}>
        {info.label}
      </span>
    </div>
    <div
      class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
    >
      <span>{info.role}</span>
      {#if info.commitOid}
        <span class="font-mono">{short(info.commitOid)}</span>
      {/if}
    </div>
  </div>

  <div class="min-h-0 flex-1 space-y-1.5 overflow-y-auto p-2 text-xs">
    {#each segments as seg (seg.id)}
      {@const choice = draft.choices[seg.id] ?? "unresolved"}
      {@const applied = Boolean(draft.appliedNonConflicting[seg.id])}
      {@const lines = isLeft ? seg.leftLines : seg.rightLines}
      {@const lineStart = isLeft ? seg.leftStart : seg.rightStart}

      {#if seg.kind === "common"}
        <div class="rounded-sm px-2 py-1 font-mono text-muted-foreground">
          {#each lines as line, idx (`${seg.id}:${side}:${idx}`)}
            <div class="flex gap-2">
              <span class="w-7 shrink-0 text-right select-none opacity-60">
                {lineStart + idx}
              </span>
              <span class="min-w-0 flex-1 whitespace-pre-wrap break-all"
                >{line || " "}</span
              >
            </div>
          {/each}
        </div>
      {:else if seg.kind === "conflict"}
        <div
          class="space-y-1.5 rounded-md border border-destructive/40 bg-destructive/5 p-2"
          data-conflict-segment={isLeft ? seg.id : undefined}
          data-active-conflict={isLeft
            ? activeConflictSegmentId === seg.id
            : undefined}
        >
          <div class="flex flex-wrap items-center justify-between gap-1">
            {#if isLeft}
              <Badge variant={choice === "unresolved" ? "destructive" : "info"}>
                冲突块 · 左侧 ({lines.length} 行)
              </Badge>
            {/if}
            <div class="flex flex-wrap items-center gap-1">
              <Button
                size="xs"
                variant="outline"
                aria-pressed={choice === primaryChoice}
                onclick={() => onChooseChunk(seg.id, primaryChoice)}
                data-testid={`conflict-accept-${side}-${seg.id}`}
              >
                {#if !isLeft}<IconArrowLeft aria-hidden="true" />{/if}
                {isLeft ? "接受左侧" : "接受右侧"}
                {#if isLeft}<IconArrowRight aria-hidden="true" />{/if}
              </Button>
              <Button
                size="xs"
                variant="ghost"
                aria-pressed={choice === bothChoice}
                title={isLeft
                  ? "先放入左侧行，再紧接放入右侧行"
                  : "先放入右侧行，再紧接放入左侧行"}
                onclick={() => onChooseChunk(seg.id, bothChoice)}
              >
                {#if !isLeft}<IconChevronsLeft aria-hidden="true" />{/if}
                {isLeft ? "先左后右" : "先右后左"}
                {#if isLeft}<IconChevronsRight aria-hidden="true" />{/if}
              </Button>
            </div>
            {#if !isLeft}
              <Badge variant={choice === "unresolved" ? "destructive" : "info"}>
                冲突块 · 右侧 ({lines.length} 行)
              </Badge>
            {/if}
          </div>

          {#if showBase}
            <div
              class="rounded-sm border bg-background p-1.5 font-mono text-muted-foreground"
            >
              <div class="mb-0.5 font-sans text-xs font-medium">
                共同祖先 Base ({seg.baseLines.length} 行)：
              </div>
              {#if seg.baseLines.length === 0}
                <div class="italic">(Base 中无行)</div>
              {:else}
                {#each seg.baseLines as bLine, bIdx (`${seg.id}:${side}b:${bIdx}`)}
                  <div class="whitespace-pre-wrap break-all">
                    {bLine || " "}
                  </div>
                {/each}
              {/if}
            </div>
          {/if}

          <div class="font-mono text-foreground">
            {#if lines.length === 0}
              <div class="italic text-muted-foreground">
                ({isLeft ? "左侧" : "右侧"}删除此段)
              </div>
            {:else}
              {#each lines as line, idx (`${seg.id}:${side}c:${idx}`)}
                <div class="flex gap-2">
                  <span
                    class="w-7 shrink-0 text-right text-muted-foreground select-none"
                  >
                    {lineStart + idx}
                  </span>
                  <span class="min-w-0 flex-1 whitespace-pre-wrap break-all"
                    >{line || " "}</span
                  >
                </div>
              {/each}
            {/if}
          </div>
        </div>
      {:else if seg.kind === changeKind || seg.kind === "change_both_same"}
        <div class="space-y-1 rounded-md border border-info/40 bg-info/5 p-2">
          <div class="flex flex-wrap items-center justify-between gap-1">
            {#if isLeft}
              <Badge variant="info">
                {seg.kind === "change_both_same"
                  ? "两侧相同改动"
                  : "左侧非冲突改动"}
              </Badge>
            {/if}
            <Button
              size="xs"
              variant="outline"
              aria-pressed={applied}
              onclick={() => onToggleNonConflicting(seg.id)}
            >
              {#if !isLeft}<IconArrowLeft aria-hidden="true" />{/if}
              {applied ? "已应用到结果" : "应用此改动"}
              {#if isLeft}<IconArrowRight aria-hidden="true" />{/if}
            </Button>
            {#if !isLeft}
              <Badge variant="info">
                {seg.kind === "change_both_same"
                  ? "两侧相同改动"
                  : "右侧非冲突改动"}
              </Badge>
            {/if}
          </div>
          <div class="font-mono text-foreground">
            {#if lines.length === 0}
              <div class="italic text-muted-foreground">(删除 Base 行)</div>
            {:else}
              {#each lines as line, idx (`${seg.id}:${side}n:${idx}`)}
                <div class="whitespace-pre-wrap break-all">{line || " "}</div>
              {/each}
            {/if}
          </div>
        </div>
      {/if}
    {/each}
  </div>
</section>
