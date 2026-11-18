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
    showSessionSwitcher,
    controls,
    byBranch,
    onGroupingChange,
  }: {
    oncollapse?: () => void;
    onclose?: () => void;
    showSessionSwitcher: boolean;
    controls: string;
    byBranch: boolean;
    onGroupingChange?: (byBranch: boolean) => void;
  } = $props();
</script>

<div class="sidebar-header-frame">
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
    <span class="brand flex-1" role="img" aria-label="WebMux" title="WebMux">
      <Brand />
    </span>
    {#if onGroupingChange}
      <div class="grouping">
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
</div>

<style>
  .sidebar-header-frame {
    container: sidebar-header / inline-size;
  }

  .sidebar-header {
    --control-sm: 32px;
    --control-touch: 32px;
    padding-inline: var(--sidebar-gutter);
    flex-wrap: wrap;
    row-gap: 4px;
  }

  .drawer {
    --control-touch: 44px;
  }

  .grouping {
    display: flex;
    flex: none;
    max-inline-size: 100%;
    justify-content: flex-end;
  }

  /* Keep spelled-out English labels when large text needs another row. */
  @container sidebar-header (width < calc(138px + 6.3rem)) {
    .sidebar-header:lang(en):not(.drawer) .grouping {
      flex-basis: 100%;
      order: 1;
    }
  }

  @container sidebar-header (width < calc(116px + 6.3rem)) {
    .drawer:lang(en) .grouping {
      flex-basis: 100%;
      order: 1;
    }
  }

  /* Leave enough width for both labels and the adjacent actions before showing icons. */
  @container sidebar-header (width < calc(144px + 5rem)) {
    .sidebar-header:not(.drawer) :global([data-grouping-icon]) {
      display: none;
    }
  }

  @container sidebar-header (width < calc(110px + 5rem)) {
    .drawer :global([data-grouping-icon]) {
      display: none;
    }
  }

  @container sidebar-header (width < calc(169px + 8rem)) {
    .sidebar-header:lang(en):not(.drawer) :global([data-grouping-icon]) {
      display: none;
    }
  }

  @container sidebar-header (width < calc(116px + 8rem)) {
    .drawer:lang(en) :global([data-grouping-icon]) {
      display: none;
    }
  }

  .brand {
    min-inline-size: 28px;
    margin-inline-end: 12px;
    gap: 0;
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
    padding-inline-start: 6px;
  }

  @container brand-wordmark (width < calc(8px + 3.5rem)) {
    .brand :global([data-brand-name] > span) {
      display: none;
    }
  }
</style>
