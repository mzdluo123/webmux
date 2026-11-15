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
  import IconDots from "~icons/tabler/dots";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconHistory from "~icons/tabler/history";
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconLayoutSidebarLeftExpand from "~icons/tabler/layout-sidebar-left-expand";
  import IconPhoto from "~icons/tabler/photo";
  import IconPictureInPictureOff from "~icons/tabler/picture-in-picture-off";
  import IconPictureInPictureOn from "~icons/tabler/picture-in-picture-on";
  import IconRefresh from "~icons/tabler/refresh";
  import IconX from "~icons/tabler/x";

  let {
    navOpen = $bindable(),
    navStatus,
    tabs,
    context,
    pending,
    popout,
    activeMenu,
    section,
    error,
    dropStatus,
    beginOpen,
    supersedeOpenOnTabKey,
    close,
    refreshAll,
    inspectablePath = "",
    onOpenGit,
    onInspectGitFile,
  }: {
    navOpen: boolean;
    navStatus: string;
    tabs: CodeWorkspaceTab[];
    context?: CodeContext;
    pending: boolean;
    popout?: { popped: boolean; toggle: () => void };
    activeMenu?: Snippet;
    section?: HTMLElement;
    error: string;
    dropStatus: string;
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
</script>

<div class="flex shrink-0 items-stretch border-b">
  <div class="flex shrink-0 items-center px-2 py-1">
    <Button
      variant="ghost"
      size="icon-sm"
      class="relative"
      aria-label={navOpen ? m.s5_collapse_nav() : m.s5_file_navigation()}
      aria-expanded={navOpen}
      title={navOpen
        ? m.s5_collapse_nav()
        : navStatus
          ? m.s5_file_navigation_with_status({ status: navStatus })
          : m.s5_file_navigation()}
      onclick={() => (navOpen = !navOpen)}
      >{#if navOpen}<IconLayoutSidebarLeftCollapse
          aria-hidden="true"
        />{:else}<IconLayoutSidebarLeftExpand
          aria-hidden="true"
        />{/if}{#if navStatus && !navOpen}<span
          class="absolute top-0.5 right-0.5 size-1.5 rounded-full bg-warning"
          aria-hidden="true"
        ></span>{/if}</Button
    >
  </div>
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
              title={tab.rootKind === "artifacts"
                ? m.s5_artifacts_tab_title({ path })
                : path}
              onclick={beginOpen}
              onkeydown={supersedeOpenOnTabKey}
            >
              {#snippet child({ props })}
                <ListButton {...props} class={undefined} variant="tab"
                  >{#if image}<IconPhoto
                      class="mr-1 inline size-3.5 align-[-0.125em]"
                      aria-hidden="true"
                    />{/if}{path.slice(path.lastIndexOf("/") + 1)} · {meta}</ListButton
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
  <div class="flex shrink-0 items-center gap-1 px-2 py-1">
    {#if onInspectGitFile && inspectablePath}<DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}<Button
              {...props}
              variant="ghost"
              size="icon-sm"
              aria-label={m.s5_file_git_history_and_blame()}
              title={m.s5_file_inspector_title({ path: inspectablePath })}
              data-testid="code-git-file-inspector-trigger"
              ><IconHistory aria-hidden="true" /></Button
            >{/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content
          align="end"
          collisionPadding={safeAreaPadding(section)}
          class="w-56 max-w-[calc(100vw-1rem)]"
        >
          <DropdownMenu.Item
            onclick={() => onInspectGitFile(inspectablePath, "history")}
          >
            {m.s5_file_history_follow()}
          </DropdownMenu.Item>
          <DropdownMenu.Item
            onclick={() => onInspectGitFile(inspectablePath, "blame")}
          >
            {m.s5_line_blame()}
          </DropdownMenu.Item>
          <DropdownMenu.Item
            onclick={() => onInspectGitFile(inspectablePath, "compare")}
          >
            {m.s5_compare_with_ref()}
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Root>{/if}
    {#if onOpenGit}<Button
        variant="ghost"
        size="icon-sm"
        aria-label={m.s5_view_in_git()}
        title={m.s5_view_in_git()}
        onclick={onOpenGit}><IconGitBranch aria-hidden="true" /></Button
      >{/if}
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={m.s5_refresh_all()}
      title={context?.rootPath
        ? m.s5_refresh_all_root({ path: context.rootPath })
        : m.s5_refresh_all()}
      disabled={pending}
      onclick={refreshAll}><IconRefresh aria-hidden="true" /></Button
    >
    {#if popout}<Button
        variant="ghost"
        size="icon-sm"
        class="max-sm:hidden"
        aria-label={popout.popped
          ? m.s5_dock_code_window()
          : m.s5_open_code_in_popout()}
        aria-pressed={popout.popped}
        title={popout.popped
          ? m.s5_dock_code_window()
          : m.s5_open_code_in_popout()}
        onclick={() => popout?.toggle()}
        >{#if popout.popped}<IconPictureInPictureOff
            aria-hidden="true"
          />{:else}<IconPictureInPictureOn aria-hidden="true" />{/if}</Button
      >{/if}
    {#if activeMenu}<DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}<Button
              {...props}
              variant="ghost"
              size="icon-sm"
              class="sm:hidden"
              aria-label={m.s5_code_menu()}
              title={m.s5_code_menu()}><IconDots aria-hidden="true" /></Button
            >{/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content
          align="end"
          collisionPadding={safeAreaPadding(section)}
          class="w-72 max-w-[calc(100vw-1rem)]"
        >
          {@render activeMenu()}
        </DropdownMenu.Content>
      </DropdownMenu.Root>{/if}
  </div>
</div>
{#if error}<p
    role="alert"
    class="shrink-0 px-2 py-1 text-sm break-all text-destructive"
  >
    {error}
  </p>{/if}
{#if context?.reason}<p
    class="shrink-0 px-2 py-1 text-xs text-muted-foreground"
  >
    {context.reason}
  </p>{/if}
{#if dropStatus && !navOpen}<p
    role="status"
    class="shrink-0 px-2 py-1 text-xs text-muted-foreground"
  >
    {dropStatus}
  </p>{/if}
