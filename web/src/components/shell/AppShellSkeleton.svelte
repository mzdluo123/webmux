<script lang="ts">
  import Brand from "./Brand.svelte";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import { shellUi } from "../../hooks/shared/shell.svelte";
  import { m } from "$lib/paraglide/messages.js";

  let { terminal = false }: { terminal?: boolean } = $props();
</script>

<div
  class="app-shell"
  data-collapsed={shellUi.collapsed}
  style:--app-sidebar-full-width={`${shellUi.width}px`}
>
  <div class="app-sidebar app-sidebar-rail" aria-hidden="true">
    <Skeleton class="size-9" />
    <Skeleton class="size-9" />
    <Skeleton class="size-9" />
    <Skeleton class="mt-auto mb-2 size-9" />
  </div>
  <div class="app-sidebar app-sidebar-full flex flex-col" aria-hidden="true">
    <div class="flex items-center px-3 pt-2 pb-1">
      <p class="brand"><Brand /></p>
    </div>
    <div class="flex flex-1 flex-col gap-2 px-3 pt-2">
      <Skeleton class="h-9 w-full" />
      <div class="mt-3 space-y-2">
        <Skeleton class="h-7 w-4/5" />
        <Skeleton class="h-7 w-full" />
        <Skeleton class="h-7 w-3/4" />
        <Skeleton class="h-7 w-5/6" />
      </div>
      <Skeleton class="mt-auto mb-3 h-8 w-full" />
    </div>
  </div>
  <main
    class="app-shell-main"
    aria-busy="true"
    aria-label={m.s1_boot_loading_aria()}
  >
    {#if terminal}
      <div class="terminal-page" aria-hidden="true">
        <div class="flex h-8 items-center gap-2 border-b bg-card px-2">
          <Skeleton class="h-4 w-32" />
          <Skeleton class="ml-auto h-4 w-16" />
        </div>
        <div class="flex-1 bg-background"></div>
      </div>
    {:else}
      <div class="workspace-page safe-area-page min-h-dvh" aria-hidden="true">
        <div
          class="app-header flex min-h-(--app-header-height) items-center justify-between border-b bg-card px-2 sm:hidden"
        >
          <Skeleton class="size-8" />
          <Skeleton class="size-8" />
        </div>
        <div class="mx-auto max-w-6xl space-y-4 px-4 py-5 sm:px-6 lg:px-8">
          <Skeleton class="h-40 w-full" />
          <Skeleton class="h-40 w-full" />
          <Skeleton class="h-40 w-full" />
        </div>
      </div>
    {/if}
  </main>
</div>
