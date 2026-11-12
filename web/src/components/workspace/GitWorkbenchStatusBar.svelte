<script lang="ts">
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconCheck from "~icons/tabler/check";
  import IconClock from "~icons/tabler/clock";
  import IconFolder from "~icons/tabler/folder";
  import IconHistory from "~icons/tabler/history";
  import IconLoader2 from "~icons/tabler/loader-2";
  import IconRefresh from "~icons/tabler/refresh";
  import IconX from "~icons/tabler/x";
  import { Badge, type BadgeVariant } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import type {
    GitActiveOperationState,
    GitOperation,
    GitOperationStatus,
  } from "../../contracts/git-workbench-schemas";
  import {
    gitOperationKindLabel,
    gitOperationStatusLabel,
  } from "../../domains/workspace/git-operations-model";

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
    return parsed.toLocaleTimeString("zh-CN", {
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
        return "等待校验";
      case "running":
        return "执行中";
      case "reconciling":
        return "重连核对中";
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
    return "就绪";
  });

  const fetchedLabel = $derived(
    lastFetchedAt ? formatShortTime(lastFetchedAt) : "",
  );
</script>

<footer
  class="flex h-[26px] min-w-0 shrink-0 items-center justify-between gap-2 overflow-hidden border-t border-git-border bg-git-panel px-2.5 text-xs text-muted-foreground"
  data-testid="git-workbench-status-bar"
>
  <div class="flex min-w-0 shrink items-center gap-1.5 max-sm:max-w-[35%]">
    <IconFolder
      class="size-3.5 shrink-0 text-muted-foreground"
      aria-hidden="true"
    />
    <span
      class="min-w-0 truncate font-mono"
      title={worktreePath ? `工作树：${worktreePath}` : "无工作树（只读仓库）"}
    >
      {worktreePath ?? "只读仓库"}
    </span>
  </div>

  <div
    class="flex min-w-0 flex-1 items-center justify-end gap-2 overflow-hidden"
  >
    {#if fetchedLabel}
      <span
        class="flex shrink-0 items-center gap-1 font-mono text-muted-foreground max-md:hidden"
        title={`最近获取远程：${lastFetchedAt}`}
        data-testid="git-status-last-fetched"
      >
        <IconClock class="size-3 shrink-0" aria-hidden="true" />
        FETCH {fetchedLabel}
      </span>
    {/if}

    <Popover.Root bind:open>
      <Popover.Trigger>
        {#snippet child({ props })}
          <ListButton
            {...props}
            variant="git-status-trigger"
            class="min-w-0 max-w-full overflow-hidden"
            aria-label="展开 Git 操作记录与状态详情"
            title={barSummary}
            data-testid="git-operations-popover-trigger"
          >
            {#if activeOperation && activeOperation.phase !== "idle"}
              <IconLoader2 class="animate-spin text-info" aria-hidden="true" />
              <Badge variant="info" class="max-w-full min-w-0 shrink">
                <span class="truncate">{activePhaseLabel}</span>
              </Badge>
            {:else if latestOp}
              {#if latestOp.status === "succeeded"}
                <IconCheck class="text-success" aria-hidden="true" />
              {:else if latestOp.status === "failed"}
                <IconX class="text-destructive" aria-hidden="true" />
              {:else if latestOp.status === "needs_review" || latestOp.status === "uncertain"}
                <IconAlertTriangle class="text-warning" aria-hidden="true" />
              {:else}
                <IconLoader2
                  class="animate-spin text-info"
                  aria-hidden="true"
                />
              {/if}
              <Badge
                variant={statusBadgeVariant(latestOp.status)}
                class="max-w-full min-w-0 shrink"
              >
                <span class="truncate">
                  {gitOperationKindLabel(latestOp.kind)} · {gitOperationStatusLabel(
                    latestOp.status,
                  )}
                </span>
              </Badge>
            {:else}
              <IconHistory aria-hidden="true" />
            {/if}

            <span
              class="min-w-0 flex-1 truncate text-foreground"
              role="status"
              data-testid="git-status-summary"
            >
              {barSummary}
            </span>
            {#if operations.length > 0}
              <span
                class="shrink-0 font-mono text-muted-foreground max-sm:hidden"
              >
                ({operations.length})
              </span>
            {/if}
          </ListButton>
        {/snippet}
      </Popover.Trigger>

      <Popover.Content
        side="top"
        align="end"
        collisionPadding={8}
        class="w-[min(92vw,30rem)] max-h-(--bits-popover-content-available-height) overflow-y-auto p-3"
        data-testid="git-operations-popover"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0">
            <p class="text-xs font-semibold text-foreground">
              最近 Git 写操作记录
            </p>
            <p class="truncate font-mono text-xs text-muted-foreground">
              {worktreePath ?? "仓库全局"}
              {#if lastFetchedAt}
                · 最近获取 {formatShortTime(lastFetchedAt)}
              {/if}
            </p>
          </div>
          {#if onRefreshOperations}
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label="刷新操作记录"
              title="刷新操作记录"
              onclick={onRefreshOperations}
            >
              <IconRefresh aria-hidden="true" />
            </Button>
          {/if}
        </div>

        {#if activeOperation && activeOperation.phase !== "idle"}
          <div
            class="flex items-center gap-2 rounded-md border border-git-border bg-git-panel px-2.5 py-2 text-xs"
            data-testid="git-active-operation-card"
          >
            <IconLoader2
              class="size-4 shrink-0 animate-spin text-info"
              aria-hidden="true"
            />
            <div class="min-w-0 flex-1">
              <p class="font-medium text-foreground">
                {activePhaseLabel}：{activeOperation.summary}
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
            当前工作树暂无已记录的 Git 写操作。
          </p>
        {:else}
          <div
            class="max-h-44 space-y-0.5 overflow-y-auto rounded-md border border-git-border bg-git-surface p-1"
            role="listbox"
            aria-label="最近 Git 操作列表"
          >
            {#each operations as op (op.id)}
              {@const isSelected = inspectedOp?.id === op.id}
              <ListButton
                variant="git-row"
                aria-selected={isSelected}
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

          {#if inspectedOp}
            <div
              class="space-y-2 rounded-md border border-git-border bg-git-panel p-2.5 text-xs"
              data-testid="git-operation-detail"
            >
              <div class="flex flex-wrap items-center justify-between gap-1.5">
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
                  title={`操作 ID：${inspectedOp.id}`}
                >
                  op:{inspectedOp.id.slice(0, 10)}
                </span>
              </div>

              <p class="wrap-anywhere text-foreground">
                {inspectedOp.summary || "无摘要"}
              </p>

              {#if inspectedOp.reviewReasons.length > 0}
                <ul class="list-disc space-y-0.5 pl-4 text-warning">
                  {#each inspectedOp.reviewReasons as reason, idx (`${inspectedOp.id}:r:${idx}`)}
                    <li class="wrap-anywhere">{reason}</li>
                  {/each}
                </ul>
              {/if}

              {#if inspectedOp.sanitizedOutput}
                <pre
                  class="max-h-28 overflow-auto rounded-sm border border-git-border bg-git-surface p-2 font-mono text-xs whitespace-pre-wrap wrap-anywhere text-muted-foreground"
                  data-testid="git-operation-output">{inspectedOp.sanitizedOutput}</pre>
              {/if}

              <div class="flex flex-wrap items-center justify-end gap-1.5">
                {#if inspectedOp.actualOid && onInspectCommit && inspectedOp.kind === "commit"}
                  <Button
                    size="xs"
                    variant="outline"
                    onclick={() => {
                      open = false;
                      if (inspectedOp.actualOid) {
                        onInspectCommit(inspectedOp.actualOid);
                      }
                    }}
                  >
                    在历史中定位提交
                  </Button>
                {/if}
                {#if inspectedOp.status === "uncertain" && onReloadVerify}
                  <Button
                    size="xs"
                    variant="outline"
                    onclick={() => {
                      open = false;
                      onReloadVerify();
                    }}
                  >
                    刷新历史与暂存区核实
                  </Button>
                {/if}
              </div>
            </div>
          {/if}
        {/if}
      </Popover.Content>
    </Popover.Root>
  </div>
</footer>
