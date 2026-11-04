<script lang="ts">
  import { untrack } from "svelte";
  import IconMoon from "~icons/tabler/moon";
  import IconPlugConnected from "~icons/tabler/plug-connected";
  import IconSettings from "~icons/tabler/settings";
  import { Button } from "$lib/components/ui/button";
  import Brand from "../../src/components/Brand.svelte";
  import { toggleTheme } from "../../src/theme";
  import ForwardDialog from "./ForwardDialog.svelte";
  import { sampleDetected, sampleForwards, type Forward } from "./data";

  // Prototype of the home page header with the new port-forwarding entry
  // between "新建会话" and settings, and the dialog it opens.
  let {
    dialog = true,
    empty = false,
    detection = "supported",
    editing = null,
  }: {
    dialog?: boolean;
    empty?: boolean;
    detection?: "supported" | "unsupported";
    editing?: number | null;
  } = $props();
  let open = $state(untrack(() => dialog));
  let forwards = $state<Forward[]>(
    untrack(() => (empty ? [] : structuredClone(sampleForwards))),
  );
  const label = $derived(
    forwards.length ? `端口转发（${forwards.length} 个）` : "端口转发",
  );
</script>

<div class="workspace-page min-h-dvh">
  <header
    class="app-header flex min-h-(--app-header-height) items-center border-b bg-card"
  >
    <div
      class="mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-2 px-4 py-0.5 sm:px-8"
    >
      <span class="brand"><Brand /></span>
      <div class="ml-auto flex items-center gap-1 sm:gap-2">
        <span
          class="hidden items-center gap-2 text-xs text-muted-foreground sm:flex"
          >devbox 在线<span class="status-dot status-running"></span></span
        >
        <Button size="sm">＋ 新建会话</Button>
        <Button
          variant="ghost"
          size={forwards.length ? "sm" : "icon-sm"}
          aria-label={label}
          title={label}
          aria-haspopup="dialog"
          onclick={() => (open = true)}
          ><IconPlugConnected aria-hidden="true" />{#if forwards.length}<span
              class="tabular-nums"
              aria-hidden="true">{forwards.length}</span
            >{/if}</Button
        >
        <Button variant="ghost" size="icon-sm" aria-label="设置" title="设置"
          ><IconSettings aria-hidden="true" /></Button
        >
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="切换主题"
          title="切换主题"
          onclick={toggleTheme}><IconMoon aria-hidden="true" /></Button
        >
      </div>
    </div>
  </header>
  <main
    class="mx-auto max-w-7xl px-4 py-6 text-sm text-muted-foreground sm:px-8"
  >
    会话列表（原型省略）
  </main>
</div>

{#if open}
  <ForwardDialog
    bind:forwards
    detected={sampleDetected}
    {detection}
    {editing}
    onclose={() => (open = false)}
  />
{/if}
