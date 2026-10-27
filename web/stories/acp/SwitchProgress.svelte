<script lang="ts">
  import IconCircle from "~icons/tabler/circle";
  import IconCircleCheck from "~icons/tabler/circle-check";
  import IconLoader2 from "~icons/tabler/loader-2";
  import type { Side } from "./fixtures";

  // The single-writer hand-off (D3): the previous side must exit before the
  // other one starts, and input stays disabled throughout.
  let { to, step }: { to: Side; step: number } = $props();
  const steps = $derived(
    to === "tui"
      ? ["中断当前回合", "关闭聊天会话并退出 ACP", "在终端恢复会话"]
      : [
          "中断 TUI 回合并等待空闲",
          "退出 TUI",
          "加载对话历史",
          "重新应用模型与推理强度",
        ],
  );
</script>

<div
  class="flex flex-1 flex-col items-center justify-center gap-4 p-6"
  role="status"
  aria-live="polite"
>
  <IconLoader2
    class="size-8 animate-spin text-muted-foreground"
    aria-hidden="true"
  />
  <h2 class="text-center text-base font-medium">
    正在切换到{to === "tui" ? "终端" : "聊天视图"}…
  </h2>
  <ol class="w-full max-w-xs space-y-2 text-sm">
    {#each steps as label, i (label)}
      <li
        class={["flex items-center gap-2", i > step && "text-muted-foreground"]}
      >
        {#if i < step}
          <IconCircleCheck
            class="size-4 shrink-0 text-success"
            aria-label="完成"
          />
        {:else if i === step}
          <IconLoader2
            class="size-4 shrink-0 animate-spin text-info"
            aria-label="进行中"
          />
        {:else}
          <IconCircle class="size-4 shrink-0" aria-label="等待" />
        {/if}
        <span class="min-w-0 break-words">{label}</span>
      </li>
    {/each}
  </ol>
  <p class="max-w-xs text-center text-xs text-muted-foreground">
    切换期间暂停输入；失败时停留在原来的一端并显示原因。
  </p>
</div>
