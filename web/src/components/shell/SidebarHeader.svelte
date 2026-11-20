<script lang="ts">
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import Brand from "./Brand.svelte";
  import SessionSwitcherTrigger from "./SessionSwitcherTrigger.svelte";
  import WorkbenchGroupingSwitch from "./WorkbenchGroupingSwitch.svelte";

  let {
    oncollapse,
    onclose,
    onhome,
    showSessionSwitcher,
    controls,
    byBranch,
    onGroupingChange,
  }: {
    oncollapse?: () => void;
    onclose?: () => void;
    onhome?: () => void;
    showSessionSwitcher: boolean;
    controls: string;
    byBranch: boolean;
    onGroupingChange?: (byBranch: boolean) => void;
  } = $props();
</script>

<div
  class="sidebar-header flex items-center pt-2 pb-1"
  class:drawer={!!onclose}
>
  {#if oncollapse}
    <Button
      class="shrink-0"
      variant="ghost"
      size="icon-sm"
      aria-label={m.s1_collapse_sidebar()}
      title={m.s1_collapse_sidebar()}
      onclick={oncollapse}
      ><IconLayoutSidebarLeftCollapse aria-hidden="true" /></Button
    >
  {/if}
  <a
    class="brand flex-1 rounded-md transition-opacity hover:opacity-80 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    href="/"
    aria-label="WebMux"
    title={m.s1_back_to_workspace()}
    onclick={(event) => {
      if (!onhome) return;
      event.preventDefault();
      onhome();
    }}
  >
    <Brand />
  </a>
  {#if onGroupingChange}
    <div class="shrink-0">
      <WorkbenchGroupingSwitch
        {byBranch}
        {controls}
        onchange={onGroupingChange}
      />
    </div>
  {/if}
  {#if showSessionSwitcher}
    <div class="ms-[8px] shrink-0">
      <SessionSwitcherTrigger size="icon-sm" />
    </div>
  {/if}
  {#if onclose}
    <Button
      class="ms-[8px] shrink-0"
      variant="ghost"
      size="icon-touch-sm"
      data-sidebar-close
      aria-label={m.s1_close_sidebar()}
      title={m.s1_close_sidebar()}
      onclick={onclose}><IconX aria-hidden="true" /></Button
    >
  {/if}
</div>

<style>
  .sidebar-header {
    --control-sm: 32px;
    --control-touch: 32px;
    padding-inline: var(--sidebar-gutter);
  }

  .drawer {
    --control-touch: 44px;
  }

  .brand {
    min-inline-size: 28px;
    margin-inline-end: 12px;
    gap: 0;
    font-size: 0.8125rem;
    font-weight: 600;
  }

  .brand :global(.brand-mark) {
    width: 28px;
    height: 28px;
  }

  .brand :global([data-brand-name]) {
    container: brand-wordmark / inline-size;
    flex: 1;
    min-inline-size: 0;
  }

  .brand :global([data-brand-name] > span) {
    white-space: nowrap;
    padding-inline-start: 5px;
  }

  @container brand-wordmark (width < calc(6px + 3.3rem)) {
    .brand :global([data-brand-name] > span) {
      display: none;
    }
  }
</style>
