<script lang="ts">
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    message,
    hasActiveWorktreeSession = false,
    openLabel = "跳转工作树终端",
    newLabel = "新建工作树终端",
    onOpenConflictResolver,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    message: string;
    hasActiveWorktreeSession?: boolean;
    openLabel?: string;
    newLabel?: string;
    onOpenConflictResolver?: () => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } = $props();
</script>

<div class="border-b bg-background px-3 py-2">
  <InlineNotice tone="danger">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span>{message}</span>
      <div class="flex flex-wrap items-center gap-1.5">
        {#if onOpenConflictResolver}
          <Button
            size="sm"
            variant="default"
            wrap
            onclick={onOpenConflictResolver}
            data-testid="git-notice-open-conflict-resolver-btn"
          >
            <IconArrowsSplit2 aria-hidden="true" />
            打开三方冲突解决器
          </Button>
        {/if}
        {#if hasActiveWorktreeSession && onOpenWorktreeTerminal}
          <Button
            size="sm"
            variant="outline"
            wrap
            onclick={onOpenWorktreeTerminal}
          >
            <IconTerminal2 aria-hidden="true" />
            {openLabel}
          </Button>
        {/if}
        {#if onNewWorktreeTerminal}
          <Button
            size="sm"
            variant="outline"
            wrap
            onclick={onNewWorktreeTerminal}
          >
            <IconTerminal2 aria-hidden="true" />
            {newLabel}
          </Button>
        {/if}
      </div>
    </div>
  </InlineNotice>
</div>
