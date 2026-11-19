<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Tabs from "$lib/components/ui/tabs";
  import {
    safeAreaPadding,
    type CodeContext,
    type CodeWorkspaceTab,
  } from "../../domains/code/code-workspace";
  import { fileTabLabels } from "../../domains/code/file-tab-labels";
  import IconDots from "~icons/tabler/dots";
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconLayoutSidebarLeftExpand from "~icons/tabler/layout-sidebar-left-expand";
  import IconPhoto from "~icons/tabler/photo";
  import IconX from "~icons/tabler/x";

  let {
    part = "all",
    navOpen = $bindable(),
    dockNavigation,
    navStatus,
    tabs,
    context,
    pending,
    popout,
    activeMenu,
    activeQuick,
    section,
    beginOpen,
    supersedeOpenOnTabKey,
    close,
    refreshAll,
    inspectablePath = "",
    onOpenGit,
    onInspectGitFile,
  }: {
    part?: "all" | "tabs" | "commands";
    navOpen: boolean;
    dockNavigation?: { shown: boolean; show: () => void };
    navStatus: string;
    tabs: CodeWorkspaceTab[];
    context?: CodeContext;
    pending: boolean;
    popout?: { popped: boolean; toggle: () => void };
    activeMenu?: Snippet;
    activeQuick?: Snippet;
    section?: HTMLElement;
    beginOpen: () => void;
    supersedeOpenOnTabKey: (event: KeyboardEvent) => void;
    close: (key: string) => void;
    refreshAll: () => void;
    inspectablePath?: string;
    onOpenGit?: () => void;
    onInspectGitFile?: (
      path: string,
      tab: "history" | "blame" | "compare",
    ) => void;
  } = $props();
  const labels = $derived(fileTabLabels(tabs));
</script>

{#snippet tabStrip()}
  {#if !dockNavigation?.shown}
    <div class="flex shrink-0 items-center">
      <Button
        variant="ghost"
        size="icon-sm"
        class="relative"
        aria-label={!dockNavigation && navOpen
          ? m.s5_collapse_nav()
          : m.s5_file_navigation()}
        aria-expanded={dockNavigation?.shown ?? navOpen}
        title={!dockNavigation && navOpen
          ? m.s5_collapse_nav()
          : navStatus
            ? m.s5_file_navigation_with_status({ status: navStatus })
            : m.s5_file_navigation()}
        onclick={() => {
          if (dockNavigation) dockNavigation.show();
          else navOpen = !navOpen;
        }}
        >{#if !dockNavigation && navOpen}<IconLayoutSidebarLeftCollapse
            aria-hidden="true"
          />{:else}<IconLayoutSidebarLeftExpand
            aria-hidden="true"
          />{/if}{#if navStatus && !navOpen}<span
            class="absolute top-0.5 right-0.5 size-1.5 rounded-full bg-warning"
            aria-hidden="true"
          ></span>{/if}</Button
      >
    </div>
  {/if}
  <Tabs.List variant="line" aria-label={m.s5_file_tabs()}>
    {#snippet child({ props })}
      <div
        {...props}
        class="flex min-w-0 flex-1 overflow-x-auto scrollbar-thin"
      >
        {#each tabs as tab (tab.key)}
          {@const path = tab.file?.document.path ?? tab.diff?.new.path ?? ""}
          {@const meta = `${tab.rootKind === "artifacts" ? m.s5_artifacts_folder() : tab.diff ? tab.diff.change.group : tab.file?.document.source}${tab.status !== "unchanged" ? " ●" : ""}`}
          {@const image = [
            tab.file?.document.mediaType,
            tab.diff?.new.mediaType,
            tab.diff?.old.mediaType,
          ].some((type) => type?.startsWith("image/"))}
          <div class="flex shrink-0 items-center border-r first:border-l">
            <Tabs.Trigger
              value={tab.key}
              aria-label={`${path} · ${meta}`}
              title={tab.file?.document.absolutePath ??
                tab.diff?.new.absolutePath ??
                path}
              onclick={beginOpen}
              onkeydown={supersedeOpenOnTabKey}
            >
              {#snippet child({ props })}
                <ListButton {...props} class={undefined} variant="tab"
                  >{#if image}<IconPhoto
                      class="mr-1 inline size-3.5 align-[-0.125em]"
                      aria-hidden="true"
                    />{/if}<span class="truncate"
                    >{labels.get(tab.key)}{tab.status !== "unchanged"
                      ? " ●"
                      : ""}</span
                  ></ListButton
                >
              {/snippet}
            </Tabs.Trigger>
            <ListButton
              variant="tab-close"
              aria-label={m.s5_close_tab({ path })}
              title={m.s5_close_tab({ path })}
              onclick={() => close(tab.key)}
              ><IconX aria-hidden="true" /></ListButton
            >
          </div>{/each}
      </div>
    {/snippet}
  </Tabs.List>
{/snippet}

{#snippet commands()}
  <div class="flex shrink-0 items-center">
    {#if activeQuick}<div
        class:quick-actions={part === "all"}
        class="flex items-center"
      >
        {@render activeQuick()}
      </div>{/if}
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        {#snippet child({ props })}<Button
            {...props}
            variant="ghost"
            size="icon-sm"
            aria-label={m.s5_code_menu()}
            title={m.s5_code_menu()}><IconDots aria-hidden="true" /></Button
          >{/snippet}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content
        align="end"
        collisionPadding={safeAreaPadding(section)}
        class="w-72 max-w-[calc(100vw-1rem)]"
      >
        {#if activeMenu}{@render activeMenu()}<DropdownMenu.Separator />{/if}
        {#if context?.reason}<DropdownMenu.Label
            >{context.reason}</DropdownMenu.Label
          >{/if}
        {#if onInspectGitFile && inspectablePath}
          <DropdownMenu.Item
            onSelect={() => onInspectGitFile?.(inspectablePath, "history")}
            >{m.s5_file_history_follow()}</DropdownMenu.Item
          >
          <DropdownMenu.Item
            onSelect={() => onInspectGitFile?.(inspectablePath, "blame")}
            >{m.s5_line_blame()}</DropdownMenu.Item
          >
          <DropdownMenu.Item
            onSelect={() => onInspectGitFile?.(inspectablePath, "compare")}
            >{m.s5_compare_with_ref()}</DropdownMenu.Item
          >
        {/if}
        {#if onOpenGit}<DropdownMenu.Item onSelect={onOpenGit}
            >{m.s5_view_in_git()}</DropdownMenu.Item
          >{/if}
        <DropdownMenu.Item disabled={pending} onSelect={refreshAll}
          >{m.s5_refresh_all()}</DropdownMenu.Item
        >
        {#if popout}<DropdownMenu.Item onSelect={popout.toggle}
            >{popout.popped
              ? m.s5_dock_code_window()
              : m.s5_open_code_in_popout()}</DropdownMenu.Item
          >{/if}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </div>
{/snippet}

{#if part === "commands"}
  {@render commands()}
{:else}
  <div class="reader-strip flex min-w-0 items-center">
    {@render tabStrip()}
    {#if part === "all"}{@render commands()}{/if}
  </div>
{/if}

<style>
  .reader-strip {
    flex: 1 1 0;
    container: reader-strip / inline-size;
  }
  :global(.code-panel) > .reader-strip {
    flex: none;
    block-size: var(--control-default);
  }
  .quick-actions {
    display: none;
  }
  @container reader-strip (min-width: 26rem) {
    .quick-actions {
      display: flex;
    }
  }
  @media (any-pointer: coarse), (max-width: 639px) {
    :global(.code-panel) > .reader-strip {
      block-size: var(--control-touch);
    }
  }
</style>
