<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import type { SplitDirection } from "../terminal-layout";

  let {
    busy,
    limit,
    ready,
    zoomed,
    onsplit,
    oncreate,
    onchoose,
    onclose,
    onzoom,
  }: {
    busy: boolean;
    limit: boolean;
    ready: boolean;
    zoomed: boolean;
    onsplit: (direction: SplitDirection) => void;
    oncreate: () => void;
    onchoose: () => void;
    onclose: () => void;
    onzoom?: () => void;
  } = $props();
</script>

<div class="space-y-1" role="group" aria-label="窗格">
  <ListButton
    variant="action-sheet"
    disabled={busy || limit || !ready}
    onclick={oncreate}>新建会话…</ListButton
  >
  <ListButton variant="action-sheet" disabled={busy || limit} onclick={onchoose}
    >加入已有会话…</ListButton
  >
  <Separator class="my-3" />
  <h3 class="px-3 py-2 text-xs text-muted-foreground">快速新建 Shell</h3>
  <ListButton
    variant="action-sheet"
    aria-label="左右分屏 · 新建 Shell"
    disabled={busy || limit || !ready}
    onclick={() => onsplit("horizontal")}
  >
    <span class="min-w-0 flex-1">左右分屏 · 新建 Shell</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B %</span
    >
  </ListButton>
  <ListButton
    variant="action-sheet"
    aria-label="上下分屏 · 新建 Shell"
    disabled={busy || limit || !ready}
    onclick={() => onsplit("vertical")}
  >
    <span class="min-w-0 flex-1">上下分屏 · 新建 Shell</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B "</span
    >
  </ListButton>
  {#if limit}<p class="px-3 py-2 text-xs text-muted-foreground">
      已达到 8 个窗格上限
    </p>{/if}
  {#if onzoom}
    <Separator class="my-3" />
    <ListButton
      variant="action-sheet"
      aria-label={zoomed ? "恢复分屏" : "放大窗格"}
      onclick={onzoom}
      ><span class="min-w-0 flex-1">{zoomed ? "恢复分屏" : "放大窗格"}</span
      ><span
        class="text-xs text-muted-foreground max-sm:hidden"
        aria-hidden="true">Ctrl+B z</span
      ></ListButton
    >
  {/if}
  <Separator class="my-3" />
  <ListButton
    variant="action-sheet"
    disabled={busy}
    aria-label="关闭窗格（保留会话）"
    onclick={onclose}
    ><span class="min-w-0 flex-1">关闭窗格（保留会话）</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B x</span
    ></ListButton
  >
</div>
