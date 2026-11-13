<script lang="ts">
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    message,
    hasActiveWorktreeSession = false,
    openLabel = "跳转工作树终端",
    newLabel = "新建工作树终端",
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    message: string;
    hasActiveWorktreeSession?: boolean;
    openLabel?: string;
    newLabel?: string;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } = $props();
</script>

<div class="border-b border-git-border bg-git-panel px-3 py-2">
  <InlineNotice tone="danger">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span>{message}</span>
      <div class="flex flex-wrap items-center gap-1.5">
        {#if hasActiveWorktreeSession && onOpenWorktreeTerminal}
          <Button size="xs" variant="outline" onclick={onOpenWorktreeTerminal}>
            <IconTerminal2 aria-hidden="true" />
            {openLabel}
          </Button>
        {/if}
        {#if onNewWorktreeTerminal}
          <Button
            size="xs"
            variant="git-primary"
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
