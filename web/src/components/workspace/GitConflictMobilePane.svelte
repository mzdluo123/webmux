<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { GitConflictFileDetail } from "../../contracts/schemas";
  import type {
    GitConflictChunkChoice,
    GitConflictDraftState,
    GitConflictProgressSummary,
  } from "../../domains/workspace/git-conflict-model";

  let {
    detail,
    draft,
    progress,
    onChooseChunk,
    onApplyAllNonConflicting,
    onConfirmManualEdits,
    onChangeResultText,
  }: {
    detail: GitConflictFileDetail;
    draft: GitConflictDraftState;
    progress: GitConflictProgressSummary;
    onChooseChunk: (segmentId: string, choice: GitConflictChunkChoice) => void;
    onApplyAllNonConflicting: () => void;
    onConfirmManualEdits: () => void;
    onChangeResultText: (nextText: string) => void;
  } = $props();

  let sourceSide = $state<"left" | "right" | "base">("left");

  const activeSide = $derived(
    sourceSide === "left"
      ? detail.left
      : sourceSide === "right"
        ? detail.right
        : detail.base,
  );
  const conflictSegs = $derived(
    detail.segments.filter((s) => s.kind === "conflict"),
  );
</script>

<ScrollArea
  class="min-h-8 min-w-0 flex-1"
  data-testid="git-conflict-mobile-pane"
>
  <div class="space-y-3 p-2.5 text-xs">
    <!-- 1. Single-source read-only switcher (Left / Right / Common Ancestor Base) -->
    <div class="space-y-2 rounded-lg border bg-background p-2.5">
      <SegmentedControl.Root
        wrap
        value={sourceSide}
        onValueChange={(val) => {
          if (val === "left" || val === "right" || val === "base") {
            sourceSide = val;
          }
        }}
        aria-label="来源版本对照切换"
      >
        <SegmentedControl.Item value="left">左侧来源</SegmentedControl.Item>
        <SegmentedControl.Item value="right">右侧来源</SegmentedControl.Item>
        <SegmentedControl.Item value="base">共同祖先 Base</SegmentedControl.Item
        >
      </SegmentedControl.Root>

      <div class="space-y-1">
        <p class="font-semibold wrap-anywhere text-foreground">
          {activeSide.label}
        </p>
        <p class="wrap-anywhere text-muted-foreground">{activeSide.role}</p>
      </div>

      <div class="rounded-md border bg-background">
        <ScrollArea viewportClasses="max-h-36">
          <pre
            class="p-2 font-mono text-xs whitespace-pre-wrap break-all text-foreground">{activeSide.present
              ? activeSide.text || "(空文件)"
              : "(此侧不存在该文件)"}</pre>
        </ScrollArea>
      </div>
    </div>

    <!-- 2. Non-conflicting quick action + Per-conflict decision cards -->
    {#if progress.pendingNonConflicting > 0}
      <div
        class="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-background p-2.5"
      >
        <span class="min-w-0 flex-1 wrap-anywhere text-foreground">
          检测到 {progress.pendingNonConflicting} 处无冲突单侧改动可自动合入结果
        </span>
        <Button
          size="sm"
          variant="outline"
          wrap
          onclick={onApplyAllNonConflicting}
        >
          应用全部无冲突改动
        </Button>
      </div>
    {/if}

    {#if conflictSegs.length > 0}
      <div class="space-y-2">
        {#each conflictSegs as seg, idx (seg.id)}
          {@const choice = draft.choices[seg.id] ?? "unresolved"}
          <div
            class="space-y-2 rounded-lg border bg-background p-2.5"
            data-mobile-conflict-card={seg.id}
          >
            <div class="flex flex-wrap items-center justify-between gap-1.5">
              <span class="font-semibold text-foreground">
                冲突块 #{idx + 1} / {conflictSegs.length}
              </span>
              <Badge
                variant={choice === "unresolved" ? "destructive" : "success"}
              >
                {choice === "unresolved" ? "待决策" : "已选择"}
              </Badge>
            </div>

            <div class="space-y-1.5">
              <div class="rounded-md border bg-background p-2">
                <div class="mb-1 font-medium text-muted-foreground">
                  左侧 ({detail.left.refName || "当前分支"})：
                </div>
                <pre
                  class="font-mono text-xs whitespace-pre-wrap break-all text-foreground">{seg
                    .leftLines.length > 0
                    ? seg.leftLines.join("\n")
                    : "(删除此段)"}</pre>
              </div>

              <div class="rounded-md border bg-background p-2">
                <div class="mb-1 font-medium text-muted-foreground">
                  右侧 ({detail.right.refName || "合入来源"})：
                </div>
                <pre
                  class="font-mono text-xs whitespace-pre-wrap break-all text-foreground">{seg
                    .rightLines.length > 0
                    ? seg.rightLines.join("\n")
                    : "(删除此段)"}</pre>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-1.5">
              <Button
                size="sm"
                variant="outline"
                wrap
                aria-pressed={choice === "left"}
                onclick={() => onChooseChunk(seg.id, "left")}
              >
                接受左侧
              </Button>
              <Button
                size="sm"
                variant="outline"
                wrap
                aria-pressed={choice === "right"}
                onclick={() => onChooseChunk(seg.id, "right")}
              >
                接受右侧
              </Button>
              <Button
                size="sm"
                variant="ghost"
                wrap
                aria-pressed={choice === "both_lr"}
                onclick={() => onChooseChunk(seg.id, "both_lr")}
              >
                先左后右
              </Button>
              <Button
                size="sm"
                variant="ghost"
                wrap
                aria-pressed={choice === "both_rl"}
                onclick={() => onChooseChunk(seg.id, "both_rl")}
              >
                先右后左
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}

    <!-- 3. Continuously visible Editable Result area (never hidden behind tabs) -->
    <section
      class="space-y-2 rounded-lg border bg-background p-2.5"
      aria-label="移动端常驻合并结果预览与编辑"
      data-testid="git-conflict-mobile-result"
    >
      <div class="flex flex-wrap items-center justify-between gap-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="success">合并结果 Result</Badge>
          <span class="font-mono break-all text-foreground">{detail.path}</span>
        </div>
        {#if progress.hasSavedToDisk}
          <Badge variant="success">已保存至工作树</Badge>
        {:else}
          <Badge variant="warning">草稿未保存</Badge>
        {/if}
      </div>

      {#if draft.manualEdited && progress.unresolvedConflicts > 0 && !progress.hasConflictMarkers}
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">已直接修改结果文本：</span>
          <Button
            size="sm"
            variant="outline"
            wrap
            onclick={onConfirmManualEdits}
          >
            <IconCheck aria-hidden="true" />
            确认手动编辑已解决冲突
          </Button>
        </div>
      {/if}

      <div
        class="rounded-md border bg-background p-1.5 focus-within:ring-2 focus-within:ring-ring/40"
      >
        <Textarea
          variant="bare"
          code
          aria-label="合并结果文本编辑器"
          value={draft.resultText}
          oninput={(e) => onChangeResultText(e.currentTarget.value)}
          class="min-h-40"
          data-testid="git-conflict-mobile-textarea"
        />
      </div>
    </section>
  </div>
</ScrollArea>
