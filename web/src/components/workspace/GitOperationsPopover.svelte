<script lang="ts">
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconHistory from "~icons/tabler/history";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconRefresh from "~icons/tabler/refresh";
  import IconX from "~icons/tabler/x";
  import { Badge, type BadgeVariant } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { locale } from "$lib/locale";
  import * as m from "$lib/paraglide/messages.js";
  import type {
    GitActiveOperationState,
    GitOperation,
    GitOperationStatus,
  } from "../../contracts/git-workbench-schemas";
  import {
    gitOperationKindLabel,
    gitOperationStatusLabel,
  } from "../../domains/workspace/git-operations-model";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  let {
    worktreePath = null,
    statusBanner = "",
    lastFetchedAt,
    activeOperation = null,
    operations = [],
    onInspectCommit,
    onRefreshOperations,
    onReloadVerify,
  }: {
    worktreePath?: string | null;
    statusBanner?: string;
    lastFetchedAt?: string | null;
    activeOperation?: GitActiveOperationState | null;
    operations?: readonly GitOperation[];
    onInspectCommit?: (commitOid: string) => void;
    onRefreshOperations?: () => void;
    onReloadVerify?: () => void;
  } = $props();

  let open = $state(false);
  let selectedOpId = $state<string | null>(null);

  const latestOp = $derived(operations[0] ?? null);
  const inspectedOp = $derived(
    (selectedOpId
      ? operations.find((op) => op.id === selectedOpId)
      : undefined) ?? latestOp,
  );

  function statusBadgeVariant(status: GitOperationStatus): BadgeVariant {
    switch (status) {
      case "succeeded":
        return "success";
      case "needs_review":
      case "uncertain":
        return "warning";
      case "failed":
        return "destructive";
      case "pending":
      case "running":
        return "info";
    }
  }

  function formatShortTime(iso: string | null | undefined): string {
    if (!iso) return "";
    const parsed = new Date(iso);
    if (Number.isNaN(parsed.getTime())) return "";
    return parsed.toLocaleTimeString(locale.current, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }

  const activePhaseLabel = $derived.by(() => {
    if (!activeOperation || activeOperation.phase === "idle") return "";
    switch (activeOperation.phase) {
      case "validating":
        return m.s6_ops_phase_validating();
      case "running":
        return m.s6_ops_phase_running();
      case "reconciling":
        return m.s6_ops_phase_reconciling();
    }
  });

  const barSummary = $derived.by(() => {
    if (activeOperation && activeOperation.phase !== "idle") {
      return activeOperation.summary;
    }
    if (statusBanner) return statusBanner;
    if (latestOp) {
      return (
        latestOp.summary ||
        `${gitOperationKindLabel(latestOp.kind)} · ${gitOperationStatusLabel(latestOp.status)}`
      );
    }
    return m.s6_ops_ready();
  });
  const hasActivePhase = $derived(
    Boolean(activeOperation && activeOperation.phase !== "idle"),
  );
  const hasAbnormalLatest = $derived(
    Boolean(!hasActivePhase && latestOp && latestOp.status !== "succeeded"),
  );
  const isIconOnly = $derived(
    !hasActivePhase && !hasAbnormalLatest && operations.length === 0,
  );
</script>

<div class="min-w-0 max-w-full shrink">
  <Popover.Root bind:open>
    <Popover.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="outline"
          size={isIconOnly ? "icon-sm" : "sm"}
          class="max-w-full min-w-0 shrink"
          aria-label={m.s6_ops_trigger_aria()}
          title={barSummary}
          data-testid="git-operations-popover-trigger"
        >
          {#if hasActivePhase}
            <IconLoader2 class="animate-spin text-info" aria-hidden="true" />
            <span
              class="max-w-40 truncate sm:max-w-56"
              role="status"
              data-testid="git-status-summary"
            >
              {activePhaseLabel}
            </span>
          {:else if latestOp && latestOp.status !== "succeeded"}
            {#if latestOp.status === "failed"}
              <IconX class="text-destructive" aria-hidden="true" />
            {:else if latestOp.status === "needs_review" || latestOp.status === "uncertain"}
              <IconAlertTriangle class="text-warning" aria-hidden="true" />
            {:else}
              <IconLoader2 class="animate-spin text-info" aria-hidden="true" />
            {/if}
            <span
              class="max-w-44 truncate sm:max-w-56"
              role="status"
              data-testid="git-status-summary"
            >
              {gitOperationKindLabel(latestOp.kind)} · {gitOperationStatusLabel(
                latestOp.status,
              )}
            </span>
          {:else}
            <IconHistory aria-hidden="true" />
          {/if}

          {#if operations.length > 0}
            <span
              class="shrink-0 font-mono text-muted-foreground max-sm:hidden"
            >
              ({operations.length})
            </span>
          {/if}
        </Button>
      {/snippet}
    </Popover.Trigger>

    <Popover.Content
      side="bottom"
      align="end"
      collisionPadding={8}
      class="w-[min(92vw,30rem)] p-0"
      data-testid="git-operations-popover"
    >
      <ScrollArea
        viewportClasses="max-h-(--bits-popover-content-available-height)"
      >
        <div class="p-3">
          <div class="flex items-center justify-between gap-2">
            <div class="min-w-0">
              <p class="text-xs font-semibold text-foreground">
                {m.s6_ops_recent_heading()}
              </p>
              <p class="truncate font-mono text-xs text-muted-foreground">
                {worktreePath ?? m.s6_ops_repo_global()}
                {#if lastFetchedAt}
                  {m.s6_ops_last_fetched({
                    time: formatShortTime(lastFetchedAt),
                  })}
                {/if}
              </p>
            </div>
            {#if onRefreshOperations}
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={m.s6_ops_refresh_aria()}
                title={m.s6_ops_refresh_aria()}
                onclick={onRefreshOperations}
              >
                <IconRefresh aria-hidden="true" />
              </Button>
            {/if}
          </div>

          {#if activeOperation && activeOperation.phase !== "idle"}
            <div
              class="flex items-center gap-2 rounded-md border bg-background px-2.5 py-2 text-xs"
              data-testid="git-active-operation-card"
            >
              <IconLoader2
                class="size-4 shrink-0 animate-spin text-info"
                aria-hidden="true"
              />
              <div class="min-w-0 flex-1">
                <p class="font-medium text-foreground">
                  {m.s6_ops_active_summary({
                    phase: activePhaseLabel,
                    summary: activeOperation.summary,
                  })}
                </p>
                {#if activeOperation.operationId || activeOperation.clientRequestId}
                  <p class="truncate font-mono text-muted-foreground">
                    {activeOperation.operationId
                      ? `op:${activeOperation.operationId.slice(0, 12)}`
                      : `req:${activeOperation.clientRequestId}`}
                  </p>
                {/if}
              </div>
            </div>
          {/if}

          {#if operations.length === 0}
            <p class="py-3 text-center text-xs text-muted-foreground">
              {m.s6_ops_empty()}
            </p>
          {:else}
            <div
              class="rounded-md border bg-background"
              role="region"
              aria-label={m.s6_ops_list_region_aria()}
            >
              <ScrollArea viewportClasses="max-h-44">
                <div class="space-y-0.5 p-1">
                  {#each operations as op (op.id)}
                    {@const isSelected = inspectedOp?.id === op.id}
                    <ListButton
                      variant="compact-row"
                      aria-current={isSelected || undefined}
                      onclick={() => (selectedOpId = op.id)}
                      data-operation-id={op.id}
                    >
                      <Badge variant={statusBadgeVariant(op.status)}>
                        {gitOperationKindLabel(op.kind)}
                      </Badge>
                      <Badge variant="outline">
                        {gitOperationStatusLabel(op.status)}
                      </Badge>
                      <span class="min-w-0 flex-1 truncate text-foreground">
                        {op.summary || op.subject || op.id.slice(0, 8)}
                      </span>
                      <span class="shrink-0 font-mono text-muted-foreground">
                        {formatShortTime(op.finishedAt ?? op.startedAt)}
                      </span>
                    </ListButton>
                  {/each}
                </div>
              </ScrollArea>
            </div>

            {#if inspectedOp}
              <div
                class="space-y-2 rounded-md border bg-background p-2.5 text-xs"
                data-testid="git-operation-detail"
              >
                <div
                  class="flex flex-wrap items-center justify-between gap-1.5"
                >
                  <div class="flex flex-wrap items-center gap-1.5">
                    <Badge variant={statusBadgeVariant(inspectedOp.status)}>
                      {gitOperationKindLabel(inspectedOp.kind)} · {gitOperationStatusLabel(
                        inspectedOp.status,
                      )}
                    </Badge>
                    {#if inspectedOp.actualOid}
                      <span class="font-mono text-foreground">
                        {inspectedOp.actualOid.slice(0, 8)}
                      </span>
                    {/if}
                  </div>
                  <span
                    class="font-mono text-muted-foreground"
                    title={m.s6_ops_id_tooltip({ id: inspectedOp.id })}
                  >
                    op:{inspectedOp.id.slice(0, 10)}
                  </span>
                </div>

                <p class="wrap-anywhere text-foreground">
                  {inspectedOp.summary || m.s6_ops_no_summary()}
                </p>

                {#if inspectedOp.recoveryOid || inspectedOp.stashOid}
                  <p class="font-mono break-all text-muted-foreground">
                    {#if inspectedOp.recoveryOid}
                      {m.s6_ops_recovery_oid({ oid: inspectedOp.recoveryOid })}
                    {:else if inspectedOp.stashOid}
                      {m.s6_ops_stash_oid({ oid: inspectedOp.stashOid })}
                    {/if}
                  </p>
                {/if}

                {#if inspectedOp.recoveryCommand}
                  <GitRecoveryCommandBox
                    label={m.s6_ops_recovery_cmd_label()}
                    copyLabel={m.s6_ops_copy_recovery_cmd()}
                    command={inspectedOp.recoveryCommand}
                    testId="git-operation-recovery-cmd"
                  />
                {/if}

                {#if inspectedOp.reviewReasons.length > 0}
                  <ul class="list-disc space-y-0.5 pl-4 text-warning">
                    {#each inspectedOp.reviewReasons as reason, idx (`${inspectedOp.id}:r:${idx}`)}
                      <li class="wrap-anywhere">{reason}</li>
                    {/each}
                  </ul>
                {/if}

                {#if inspectedOp.sanitizedOutput}
                  <div class="rounded-sm border bg-background">
                    <ScrollArea viewportClasses="max-h-28">
                      <pre
                        class="p-2 font-mono text-xs whitespace-pre-wrap wrap-anywhere text-muted-foreground"
                        data-testid="git-operation-output">{inspectedOp.sanitizedOutput}</pre>
                    </ScrollArea>
                  </div>
                {/if}

                <div class="flex flex-wrap items-center justify-end gap-1.5">
                  {#if inspectedOp.actualOid && onInspectCommit && inspectedOp.kind === "commit"}
                    <Button
                      size="sm"
                      variant="outline"
                      onclick={() => {
                        open = false;
                        if (inspectedOp.actualOid) {
                          onInspectCommit(inspectedOp.actualOid);
                        }
                      }}
                    >
                      {m.s6_ops_locate_commit_in_log()}
                    </Button>
                  {/if}
                  {#if inspectedOp.status === "uncertain" && onReloadVerify}
                    <Button
                      size="sm"
                      variant="outline"
                      onclick={() => {
                        open = false;
                        onReloadVerify();
                      }}
                    >
                      {m.s6_reload_verify_btn()}
                    </Button>
                  {/if}
                </div>
              </div>
            {/if}
          {/if}
        </div>
      </ScrollArea>
    </Popover.Content>
  </Popover.Root>
</div>
