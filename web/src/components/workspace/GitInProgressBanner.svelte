<script lang="ts">
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconCheck from "~icons/tabler/check";
  import IconPlayerPlay from "~icons/tabler/player-play";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitInProgressOperation } from "../../contracts/schemas";
  import {
    formatInProgressHeadline,
    gitSequencerKindLabel,
  } from "../../domains/workspace/git-divergence-model";

  let {
    operation,
    busy = false,
    conflictResolverOpen = false,
    hasActiveWorktreeSession = false,
    onOpenConflictResolver,
    onContinue,
    onOpenAbortPreview,
    onOpenWorktreeTerminal,
  }: {
    operation: GitInProgressOperation;
    busy?: boolean;
    conflictResolverOpen?: boolean;
    hasActiveWorktreeSession?: boolean;
    onOpenConflictResolver?: (path?: string) => void;
    onContinue?: () => void;
    onOpenAbortPreview?: () => void;
    onOpenWorktreeTerminal?: () => void;
  } = $props();

  const shortOid = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const primaryBlocker = $derived(operation.continueBlockers[0]?.message ?? "");
</script>

{#if operation.active}
  <section
    class="border-b border-git-border bg-git-panel px-3 py-2 text-xs"
    aria-label="进行中的 Git 操作协调恢复横幅"
    data-testid="git-in-progress-banner"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        <Badge
          variant={operation.conflictedCount > 0 ? "destructive" : "warning"}
          class="max-w-full min-w-0"
        >
          {#if operation.conflictedCount > 0}
            <IconAlertTriangle aria-hidden="true" />
            <span class="truncate">
              {gitSequencerKindLabel(operation.kind)} · {operation.conflictedCount}
              个冲突
            </span>
          {:else}
            <IconCheck aria-hidden="true" />
            <span class="truncate"
              >{gitSequencerKindLabel(operation.kind)} · 冲突已清零</span
            >
          {/if}
        </Badge>

        {#if operation.stepTotal && operation.stepTotal > 0 && operation.stepCurrent}
          <Badge variant="info" class="max-w-full min-w-0">
            <span class="truncate"
              >步骤 {operation.stepCurrent}/{operation.stepTotal}</span
            >
          </Badge>
        {/if}

        <span
          class={conflictResolverOpen
            ? "min-w-0 font-medium wrap-anywhere text-foreground max-xl:hidden"
            : "min-w-0 font-medium wrap-anywhere text-foreground"}
        >
          {formatInProgressHeadline(operation)}
        </span>

        {#if operation.stoppedCommitOid}
          <span
            class={conflictResolverOpen
              ? "font-mono text-muted-foreground max-xl:hidden"
              : "font-mono text-muted-foreground"}
          >
            暂停于 {shortOid(operation.stoppedCommitOid)}
            {#if operation.stoppedSubject}
              · {operation.stoppedSubject}
            {/if}
          </span>
        {:else if operation.origHeadOid}
          <span
            class={conflictResolverOpen
              ? "font-mono text-muted-foreground max-xl:hidden"
              : "font-mono text-muted-foreground"}
          >
            回退锚点 ORIG_HEAD {shortOid(operation.origHeadOid)}
          </span>
        {/if}
      </div>

      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if operation.conflictedCount > 0 && onOpenConflictResolver && !conflictResolverOpen}
          <Button
            size="xs"
            variant="git-primary"
            wrap
            disabled={busy}
            onclick={() =>
              onOpenConflictResolver(operation.conflictedFiles[0]?.path)}
            data-testid="git-open-conflict-resolver-btn"
          >
            <IconArrowsSplit2 aria-hidden="true" />
            打开三方冲突解决器 ({operation.conflictedCount})
          </Button>
        {/if}

        {#if onContinue && operation.kind !== "unstash"}
          <Button
            size="xs"
            variant="success"
            wrap
            disabled={!operation.canContinue || busy}
            title={!operation.canContinue
              ? primaryBlocker || "请先将所有冲突文件标记已解决后再继续操作"
              : "所有冲突已解决，继续完成当前 Git 操作"}
            onclick={onContinue}
            data-testid="git-continue-operation-btn"
          >
            <IconPlayerPlay aria-hidden="true" />
            {busy ? "正在继续…" : "继续操作 (Continue)"}
          </Button>
        {/if}

        {#if operation.canAbort && onOpenAbortPreview}
          <Button
            size="xs"
            variant="destructive-outline"
            wrap
            disabled={busy}
            onclick={onOpenAbortPreview}
            data-testid="git-abort-operation-btn"
          >
            <IconX aria-hidden="true" />
            中止操作… (Abort)
          </Button>
        {/if}

        {#if hasActiveWorktreeSession && onOpenWorktreeTerminal}
          <Button
            size="xs"
            variant="ghost"
            wrap
            onclick={onOpenWorktreeTerminal}
          >
            <IconTerminal2 aria-hidden="true" />
            终端
          </Button>
        {/if}
      </div>
    </div>

    {#if operation.conflictedCount === 0 && !operation.canContinue && primaryBlocker}
      <p class="mt-1 wrap-anywhere text-muted-foreground">
        暂不可继续：{primaryBlocker}
      </p>
    {/if}
  </section>
{/if}
