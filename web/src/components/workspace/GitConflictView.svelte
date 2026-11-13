<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconCheck from "~icons/tabler/check";
  import IconDeviceFloppy from "~icons/tabler/device-floppy";
  import IconEye from "~icons/tabler/eye";
  import IconPlayerPlay from "~icons/tabler/player-play";
  import IconRefresh from "~icons/tabler/refresh";
  import IconSparkles from "~icons/tabler/sparkles";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as Popover from "$lib/components/ui/popover";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type {
    GitConflictFileDetail,
    GitInProgressOperation,
  } from "../../contracts/schemas";
  import {
    applyAllNonConflictingSegments,
    applyConflictChunkChoice,
    createInitialConflictDraftState,
    markAllUnresolvedAsManual,
    summarizeConflictDraftProgress,
    toggleNonConflictingSegment,
    type GitBaseCompareMode,
    type GitConflictChunkChoice,
    type GitConflictDraftState,
  } from "../../domains/workspace/git-conflict-model";
  import type { GitConflictController } from "../../hooks/workspace/git-conflict-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitConflictMobilePane from "./GitConflictMobilePane.svelte";
  import GitConflictTextPane from "./GitConflictTextPane.svelte";
  import GitConflictVariantCard from "./GitConflictVariantCard.svelte";

  let {
    conflict,
    inProgress = null,
    fixtureDetail = null,
    divergenceBusy = false,
    onContinueOperation,
    onClose,
  }: {
    conflict?: GitConflictController;
    inProgress?: GitInProgressOperation | null;
    fixtureDetail?: GitConflictFileDetail | null;
    divergenceBusy?: boolean;
    onContinueOperation?: () => void;
    onClose: () => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");

  // Standalone local state when rendered from Storybook or browser unit tests via `fixtureDetail`
  let localDraft = $state<GitConflictDraftState | null>(null);
  let localBaseMode = $state<GitBaseCompareMode>("off");
  let localConflictIdx = $state(0);
  let localResolved = $state(false);
  let lastFixtureRef: GitConflictFileDetail | null = null;

  $effect(() => {
    if (conflict) return () => {};
    if (fixtureDetail !== lastFixtureRef) {
      lastFixtureRef = fixtureDetail;
      localDraft = fixtureDetail
        ? createInitialConflictDraftState(fixtureDetail)
        : null;
      localResolved = false;
      localConflictIdx = 0;
    }
    return () => {};
  });

  const detail = $derived(
    conflict ? conflict.detail : localResolved ? null : fixtureDetail,
  );
  const draft = $derived(conflict ? conflict.draft : localDraft);
  const progress = $derived(
    conflict
      ? conflict.progress
      : detail && draft
        ? summarizeConflictDraftProgress(detail, draft)
        : null,
  );
  const baseCompareMode = $derived(
    conflict ? conflict.baseCompareMode : localBaseMode,
  );
  const conflictSegs = $derived(
    detail ? detail.segments.filter((s) => s.kind === "conflict") : [],
  );
  const activeConflictIndex = $derived(
    conflict ? conflict.activeConflictIndex : localConflictIdx,
  );
  const activeConflictSegmentId = $derived(
    conflictSegs[activeConflictIndex]?.id ?? null,
  );

  const effectiveCanContinue = $derived(
    conflict ? Boolean(inProgress?.canContinue) : localResolved,
  );
  const conflictedFiles = $derived(inProgress?.conflictedFiles ?? []);

  function handleChooseChunk(segId: string, choice: GitConflictChunkChoice) {
    if (conflict) conflict.chooseChunk(segId, choice);
    else if (detail && localDraft) {
      localDraft = applyConflictChunkChoice(detail, localDraft, segId, choice);
    }
  }

  function handleToggleNonConflicting(segId: string) {
    if (conflict) conflict.toggleNonConflicting(segId);
    else if (detail && localDraft) {
      localDraft = toggleNonConflictingSegment(detail, localDraft, segId);
    }
  }

  function handleApplyAllNonConflicting() {
    if (conflict) conflict.applyAllNonConflicting();
    else if (detail && localDraft) {
      localDraft = applyAllNonConflictingSegments(detail, localDraft);
    }
  }

  function handleConfirmManual() {
    if (conflict) conflict.confirmManualEditsResolved();
    else if (localDraft) {
      localDraft = markAllUnresolvedAsManual(localDraft);
    }
  }

  function handleTextChange(nextText: string) {
    if (conflict) conflict.setResultText(nextText);
    else if (localDraft) {
      localDraft = { ...localDraft, manualEdited: true, resultText: nextText };
    }
  }

  function handleSave() {
    if (conflict) void conflict.saveResult();
    else if (localDraft) {
      localDraft = { ...localDraft, savedResultText: localDraft.resultText };
    }
  }

  function handleMarkResolved() {
    if (conflict) void conflict.markAsResolved();
    else localResolved = true;
  }

  function handleResolveVariant(
    resolution: "choose_left" | "choose_right" | "delete",
  ) {
    if (conflict) void conflict.resolveVariant(resolution);
    else localResolved = true;
  }

  function handleNavigate(delta: -1 | 1) {
    if (conflict) conflict.navigateConflict(delta);
    else if (conflictSegs.length > 0) {
      localConflictIdx =
        (localConflictIdx + delta + conflictSegs.length) % conflictSegs.length;
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col bg-git-surface"
  aria-label="三方冲突解决器"
  data-testid="git-conflict-view"
>
  <!-- Top Action & File Navigation Toolbar -->
  <div
    class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-3 py-2 text-xs"
  >
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      <Button
        size="xs"
        variant="ghost"
        onclick={onClose}
        data-testid="git-conflict-back-btn"
      >
        <IconArrowLeft aria-hidden="true" />
        返回工作台
      </Button>

      {#if conflictedFiles.length > 1 && conflict}
        <NativeSelect.Root
          size="sm"
          aria-label="切换冲突文件"
          value={conflict.activePath ?? detail?.path ?? ""}
          onchange={(e) => void conflict.selectFile(e.currentTarget.value)}
        >
          {#each conflictedFiles as cf (cf.path)}
            <NativeSelect.Option value={cf.path}>
              {cf.path} ({cf.xy})
            </NativeSelect.Option>
          {/each}
        </NativeSelect.Root>
      {:else if detail}
        <Badge variant="outline" class="max-w-full">
          <span class="truncate font-mono" title={detail.path}
            >{detail.path}</span
          >
        </Badge>
      {/if}

      {#if desktop.current && detail?.textMergeable}
        <SegmentedControl.Root
          wrap
          value={baseCompareMode}
          onValueChange={(val) => {
            if (val === "off" || val === "left_base" || val === "right_base") {
              if (conflict) conflict.setBaseCompareMode(val);
              else localBaseMode = val;
            }
          }}
          aria-label="共同祖先 Base 对照模式"
        >
          <SegmentedControl.Item value="off">隐藏 Base</SegmentedControl.Item>
          <SegmentedControl.Item value="left_base">
            左侧对照 Base
          </SegmentedControl.Item>
          <SegmentedControl.Item value="right_base">
            右侧对照 Base
          </SegmentedControl.Item>
        </SegmentedControl.Root>

        <Popover.Root>
          <Popover.Trigger>
            {#snippet child({ props })}
              <Button {...props} size="xs" variant="ghost">
                <IconEye aria-hidden="true" />
                查看完整 Base
              </Button>
            {/snippet}
          </Popover.Trigger>
          <Popover.Content class="w-[min(92vw,520px)] p-3" align="start">
            <div class="space-y-2 text-xs">
              <div class="font-semibold text-foreground">
                {detail.base.label} ({detail.base.role})
              </div>
              <pre
                class="max-h-64 overflow-auto rounded-md border border-git-border bg-git-surface p-2 font-mono text-xs whitespace-pre-wrap break-all">{detail
                  .base.present
                  ? detail.base.text || "(空文件)"
                  : "(共同祖先中无此文件)"}</pre>
            </div>
          </Popover.Content>
        </Popover.Root>
      {/if}
    </div>

    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if detail?.textMergeable && progress}
        {#if conflictSegs.length > 1}
          <Button
            size="icon-xs"
            variant="outline"
            aria-label="上一处冲突块"
            title="上一处冲突块"
            onclick={() => handleNavigate(-1)}
          >
            <IconArrowUp aria-hidden="true" />
          </Button>
          <Button
            size="icon-xs"
            variant="outline"
            aria-label="下一处冲突块"
            title="下一处冲突块"
            onclick={() => handleNavigate(1)}
          >
            <IconArrowDown aria-hidden="true" />
          </Button>
        {/if}

        <Button
          size="xs"
          variant="outline"
          wrap
          disabled={progress.pendingNonConflicting === 0}
          onclick={handleApplyAllNonConflicting}
          data-testid="git-conflict-apply-nonconflicting-btn"
        >
          <IconSparkles aria-hidden="true" />
          应用全部无冲突改动 ({progress.pendingNonConflicting})
        </Button>

        <Button
          size="xs"
          variant="outline"
          wrap
          disabled={!progress.canSaveResult ||
            conflict?.saving ||
            conflict?.resolving}
          onclick={handleSave}
          data-testid="git-conflict-save-btn"
        >
          <IconDeviceFloppy aria-hidden="true" />
          {conflict?.saving ? "正在保存…" : "保存结果 (Save Result)"}
        </Button>

        <Button
          size="xs"
          variant="git-primary"
          wrap
          disabled={!progress.canMarkResolved ||
            conflict?.saving ||
            conflict?.resolving}
          title={progress.markResolvedBlockedReason ||
            "验证工作树快照并写入 Git 暂存区 stage 0"}
          onclick={handleMarkResolved}
          data-testid="git-conflict-resolve-btn"
        >
          <IconCheck aria-hidden="true" />
          {conflict?.resolving ? "正在标记…" : "标记已解决 (Mark as Resolved)"}
        </Button>
      {/if}

      {#if onContinueOperation && inProgress?.kind !== "unstash"}
        <Button
          size="xs"
          variant="success"
          wrap
          disabled={!effectiveCanContinue || divergenceBusy}
          title={!effectiveCanContinue
            ? "请先将所有冲突文件标记已解决后再继续操作"
            : "继续完成当前 Git 操作"}
          onclick={onContinueOperation}
          data-testid="git-conflict-continue-btn"
        >
          <IconPlayerPlay aria-hidden="true" />
          {divergenceBusy ? "正在继续…" : "继续操作 (Continue)"}
        </Button>
      {/if}
    </div>
  </div>

  {#if conflict?.error}
    <div class="border-b border-git-border bg-git-panel px-3 py-2">
      <InlineNotice tone="danger">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>{conflict.error}</span>
          {#if conflict.stale || conflict.activePath}
            <Button
              size="xs"
              variant="outline"
              onclick={() => void conflict.reloadCurrent()}
            >
              <IconRefresh aria-hidden="true" />
              重新加载三方冲突快照
            </Button>
          {/if}
        </div>
      </InlineNotice>
    </div>
  {/if}

  {#if conflict?.loading && !detail}
    <div
      class="flex flex-1 items-center justify-center p-6 text-xs text-muted-foreground"
      role="status"
    >
      正在加载三方冲突快照（Base / 左侧 / 右侧 / 工作树状态）…
    </div>
  {:else if !detail}
    <div
      class="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center text-xs"
      data-testid="git-conflict-all-resolved"
    >
      <Badge variant="success">全部冲突文件已解决</Badge>
      <p class="max-w-md leading-relaxed text-muted-foreground">
        {#if inProgress?.kind === "unstash"}
          当前工作树已无未合并索引条目（stage 1..3
          已全部清零），且原贮藏条目仍安全保留在 Stash
          列表中。现在可返回「提交」视图核对暂存改动或前往「贮藏」视图手动清理。
        {:else}
          当前工作树已无未合并索引条目（stage 1..3
          已全部清零）。现在可点击「继续操作」完成此次合并、变基、遴选或还原。
        {/if}
      </p>
      <div class="flex flex-wrap items-center justify-center gap-2">
        {#if onContinueOperation && inProgress?.kind !== "unstash"}
          <Button
            size="sm"
            variant="success"
            disabled={!effectiveCanContinue || divergenceBusy}
            onclick={onContinueOperation}
          >
            <IconPlayerPlay aria-hidden="true" />
            继续操作 (Continue)
          </Button>
        {/if}
        <Button size="sm" variant="outline" onclick={onClose}>
          返回工作台
        </Button>
      </div>
    </div>
  {:else if !detail.textMergeable}
    <GitConflictVariantCard
      {detail}
      resolving={conflict?.resolving ?? false}
      onResolveVariant={handleResolveVariant}
    />
  {:else if draft && progress}
    {#if desktop.current}
      <GitConflictTextPane
        {detail}
        {draft}
        {progress}
        {baseCompareMode}
        {activeConflictSegmentId}
        onChooseChunk={handleChooseChunk}
        onToggleNonConflicting={handleToggleNonConflicting}
        onConfirmManualEdits={handleConfirmManual}
        onChangeResultText={handleTextChange}
      />
    {:else}
      <GitConflictMobilePane
        {detail}
        {draft}
        {progress}
        onChooseChunk={handleChooseChunk}
        onApplyAllNonConflicting={handleApplyAllNonConflicting}
        onConfirmManualEdits={handleConfirmManual}
        onChangeResultText={handleTextChange}
      />
    {/if}
  {/if}
</section>
