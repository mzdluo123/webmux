<script lang="ts">
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconArrowRight from "~icons/tabler/arrow-right";
  import IconChevronsLeft from "~icons/tabler/chevrons-left";
  import IconChevronsRight from "~icons/tabler/chevrons-right";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
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
  aria-label={isLeft
    ? m.s6_conflict_left_col_aria()
    : m.s6_conflict_right_col_aria()}
  data-testid={isLeft
    ? "git-conflict-left-column"
    : "git-conflict-right-column"}
>
  <div class="space-y-1 border-b bg-background px-2.5 py-2 text-xs">
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      <Badge variant={isLeft ? "info" : "outline"}>
        {isLeft ? m.s6_conflict_left_source() : m.s6_conflict_right_source()}
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

  <ScrollArea class="min-h-8 flex-1">
    <div class="space-y-1.5 p-2 text-xs">
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
                <Badge
                  variant={choice === "unresolved" ? "destructive" : "info"}
                >
                  {m.s6_conflict_left_lines_badge({
                    count: String(lines.length),
                  })}
                </Badge>
              {/if}
              <div class="flex flex-wrap items-center gap-1">
                <Button
                  size="sm"
                  variant="outline"
                  aria-pressed={choice === primaryChoice}
                  onclick={() => onChooseChunk(seg.id, primaryChoice)}
                  data-testid={`conflict-accept-${side}-${seg.id}`}
                >
                  {#if !isLeft}<IconArrowLeft aria-hidden="true" />{/if}
                  {isLeft
                    ? m.s6_conflict_accept_left()
                    : m.s6_conflict_accept_right()}
                  {#if isLeft}<IconArrowRight aria-hidden="true" />{/if}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-pressed={choice === bothChoice}
                  title={isLeft
                    ? m.s6_conflict_both_lr_tooltip()
                    : m.s6_conflict_both_rl_tooltip()}
                  onclick={() => onChooseChunk(seg.id, bothChoice)}
                >
                  {#if !isLeft}<IconChevronsLeft aria-hidden="true" />{/if}
                  {isLeft ? m.s6_conflict_both_lr() : m.s6_conflict_both_rl()}
                  {#if isLeft}<IconChevronsRight aria-hidden="true" />{/if}
                </Button>
              </div>
              {#if !isLeft}
                <Badge
                  variant={choice === "unresolved" ? "destructive" : "info"}
                >
                  {m.s6_conflict_right_lines_badge({
                    count: String(lines.length),
                  })}
                </Badge>
              {/if}
            </div>

            {#if showBase}
              <div
                class="rounded-sm border bg-background p-1.5 font-mono text-muted-foreground"
              >
                <div class="mb-0.5 font-sans text-xs font-medium">
                  {m.s6_conflict_base_lines_heading({
                    count: String(seg.baseLines.length),
                  })}
                </div>
                {#if seg.baseLines.length === 0}
                  <div class="italic">{m.s6_conflict_base_empty()}</div>
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
                  {isLeft
                    ? m.s6_conflict_left_deleted_section()
                    : m.s6_conflict_right_deleted_section()}
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
                    ? m.s6_conflict_same_change_both()
                    : m.s6_conflict_left_nonconflict()}
                </Badge>
              {/if}
              <Button
                size="sm"
                variant="outline"
                aria-pressed={applied}
                onclick={() => onToggleNonConflicting(seg.id)}
              >
                {#if !isLeft}<IconArrowLeft aria-hidden="true" />{/if}
                {applied
                  ? m.s6_conflict_applied_to_result()
                  : m.s6_conflict_apply_change()}
                {#if isLeft}<IconArrowRight aria-hidden="true" />{/if}
              </Button>
              {#if !isLeft}
                <Badge variant="info">
                  {seg.kind === "change_both_same"
                    ? m.s6_conflict_same_change_both()
                    : m.s6_conflict_right_nonconflict()}
                </Badge>
              {/if}
            </div>
            <div class="font-mono text-foreground">
              {#if lines.length === 0}
                <div class="italic text-muted-foreground">
                  {m.s6_conflict_deleted_base_lines()}
                </div>
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
  </ScrollArea>
</section>
