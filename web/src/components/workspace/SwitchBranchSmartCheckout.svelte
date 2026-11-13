<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconSwitchHorizontal from "~icons/tabler/switch-horizontal";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import type { Worktree } from "../../contracts/schemas";
  import type { createSmartCheckoutController } from "../../hooks/workspace/git-smart-checkout.svelte";
  import BlockerList from "../shared/BlockerList.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  type SmartController = ReturnType<typeof createSmartCheckoutController>;

  let {
    smart,
    worktree,
    busy = false,
    onopenconflict,
  }: {
    smart: SmartController;
    worktree: Worktree;
    busy?: boolean;
    onopenconflict?: (tree: Worktree, path?: string) => void;
  } = $props();

  const short = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const showUntrackedToggle = $derived(
    Boolean(
      worktree.hasUntracked ||
      (worktree.changes?.untracked ?? 0) > 0 ||
      (smart.preview?.untrackedCount ?? 0) > 0 ||
      smart.includeUntracked,
    ),
  );
  const nonSessionWarnings = $derived(
    (smart.preview?.warnings ?? []).filter((w) => w.code !== "sessions"),
  );
</script>

{#if smart.execError}
  <InlineNotice tone="danger">{smart.execError}</InlineNotice>
{/if}
{#if smart.conflictMessage}
  <InlineNotice tone="warning">
    {smart.conflictMessage}；已重新刷新切换预览。
  </InlineNotice>
{/if}

{#if smart.lastResult?.outcome === "rolled_back"}
  <InlineNotice tone="warning">
    <div class="space-y-1" data-testid="smart-checkout-rolled-back-notice">
      <p class="font-medium text-foreground">
        分支切换失败，已自动回滚并恢复未提交修改
      </p>
      <p class="text-xs text-muted-foreground">{smart.lastResult.summary}</p>
    </div>
  </InlineNotice>
{:else if smart.lastResult?.outcome === "conflict_needs_resolution"}
  {@const res = smart.lastResult}
  <section
    aria-label="Smart Checkout 冲突恢复指引"
    class="space-y-3 rounded-lg border border-git-border bg-git-panel p-3 text-xs"
    data-testid="smart-checkout-conflict-card"
  >
    <div class="flex flex-wrap items-center gap-2">
      <Badge variant="warning">自动恢复遇到冲突</Badge>
      {#if res.stashPreserved && res.stashOid}
        <Badge variant="outline">
          <IconArchive aria-hidden="true" />
          受控贮藏已保留 · {short(res.stashOid)}
        </Badge>
      {/if}
    </div>
    <p class="leading-relaxed text-foreground">{res.summary}</p>
    {#if res.recoveryCommand}
      <GitRecoveryCommandBox
        label="受控贮藏手动恢复命令（备份保留）"
        copyLabel="复制受控贮藏恢复命令"
        command={res.recoveryCommand}
        testId="smart-checkout-recovery-cmd"
      />
    {/if}
    {#if onopenconflict}
      <div class="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="git-primary"
          wrap
          onclick={() =>
            onopenconflict(
              res.worktree,
              res.inProgress.conflictedFiles[0]?.path,
            )}
          data-testid="smart-checkout-open-conflict-btn"
        >
          <IconArrowsSplit2 aria-hidden="true" />
          打开三方冲突解决器 ({res.inProgress.conflictedCount})
        </Button>
      </div>
    {/if}
  </section>
{/if}

{#if smart.visible && smart.lastResult?.outcome !== "conflict_needs_resolution"}
  <section
    aria-label="Smart Checkout 携带修改切换"
    class="space-y-3 rounded-lg border bg-muted/20 p-3 text-sm"
    data-testid="smart-checkout-section"
  >
    {#if smart.previewing && !smart.preview}
      <p role="status" class="text-xs text-muted-foreground">
        正在评估受控贮藏 Smart Checkout…
      </p>
    {:else if smart.previewError}
      <p role="alert" class="text-xs break-all text-destructive">
        无法评估 Smart Checkout：{smart.previewError}
      </p>
    {:else if smart.preview}
      {@const sp = smart.preview}
      {#if smart.hasActiveSessions}
        <div class="space-y-2.5">
          <p class="text-xs leading-relaxed text-muted-foreground">
            当前工作树有运行中的会话，推荐优先点击下方绿色按钮
            <strong class="text-foreground">「改为在新工作树打开」</strong
            >，避免干扰正在工作的终端或 Agent。
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            wrap
            disabled={busy}
            aria-expanded={smart.disclosureOpen}
            onclick={() => (smart.disclosureOpen = !smart.disclosureOpen)}
            data-testid="smart-checkout-session-disclosure"
          >
            {#if smart.disclosureOpen}
              <IconChevronDown aria-hidden="true" />
            {:else}
              <IconChevronRight aria-hidden="true" />
            {/if}
            仍在当前工作树原地携带修改切换（Smart Checkout）
          </Button>
        </div>
      {/if}

      {#if !smart.hasActiveSessions || smart.disclosureOpen}
        <div class="space-y-3" data-testid="smart-checkout-body">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="flex items-center gap-1.5 text-xs font-semibold">
              <IconSwitchHorizontal
                class="size-4 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              Smart Checkout（三阶段受控贮藏切换）
            </span>
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge variant="outline">已暂存 {sp.stagedCount}</Badge>
              <Badge variant="outline">未暂存 {sp.unstagedCount}</Badge>
              {#if sp.untrackedCount > 0}
                <Badge variant="outline">未跟踪 {sp.untrackedCount}</Badge>
              {/if}
            </div>
          </div>

          <p class="text-xs leading-relaxed text-muted-foreground">
            1. 创建可追踪贮藏 → 2. 切换目标分支（若失败自动回滚） → 3.
            自动恢复修改（若冲突将保留受控贮藏并进入三方冲突解决器）。
          </p>

          {#if sp.blockers.length > 0}
            <BlockerList
              title="无法执行 Smart Checkout"
              blockers={sp.blockers.map((b) => b.message)}
            />
          {/if}

          {#if nonSessionWarnings.length > 0}
            <ul class="space-y-1 text-xs text-muted-foreground">
              {#each nonSessionWarnings as w (w.code)}
                <li class="break-all">{w.message}</li>
              {/each}
            </ul>
          {/if}

          {#if showUntrackedToggle}
            <Label class="items-start">
              <Checkbox
                checked={smart.includeUntracked}
                onCheckedChange={(v) => smart.setIncludeUntracked(Boolean(v))}
                disabled={busy || smart.previewing}
                class="mt-0.5"
                data-testid="smart-checkout-include-untracked"
              />
              <span class="text-xs leading-snug">
                包含未跟踪文件（连同新建文件一并贮藏并在目标分支恢复）
              </span>
            </Label>
          {/if}

          {#if smart.hasActiveSessions && sp.blockers.length === 0}
            <Label class="items-start">
              <Checkbox
                bind:checked={smart.acknowledgeSessions}
                disabled={busy}
                class="mt-0.5"
                data-testid="smart-checkout-ack-sessions"
              />
              <span class="text-xs leading-snug">
                我已了解：Smart Checkout 会在原地暂存并恢复文件，运行中的终端或
                Agent 会话会直接看到分支切换与恢复过程。
              </span>
            </Label>
          {/if}

          <div class="flex flex-wrap items-center justify-end gap-2">
            <Button
              type="button"
              variant={smart.hasActiveSessions ? "outline" : "default"}
              size="sm"
              wrap
              disabled={!smart.canExecute}
              onclick={() => void smart.execute()}
              data-testid="smart-checkout-submit-btn"
            >
              <IconSwitchHorizontal aria-hidden="true" />
              {busy
                ? "正在携带修改切换…"
                : smart.hasActiveSessions
                  ? "确认原地 Smart Checkout"
                  : "Smart Checkout（携带修改切换）"}
            </Button>
          </div>
        </div>
      {/if}
    {/if}
  </section>
{/if}
