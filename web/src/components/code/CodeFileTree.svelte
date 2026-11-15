<script lang="ts" module>
  import type { CodeNavigatorState } from "../../contracts/schemas";
  /** The tree's part of the navigator state: open folders and their listings. */
  export type CodeFileTreeState = Pick<
    CodeNavigatorState,
    "rootOpen" | "expanded" | "branches"
  >;
</script>

<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import { onDestroy, tick, untrack } from "svelte";
  import { SvelteMap, SvelteSet } from "svelte/reactivity";
  import type { CodeListing } from "../../contracts/schemas";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconFolder from "~icons/tabler/folder";
  import IconFolderOpen from "~icons/tabler/folder-open";
  import IconFile from "~icons/tabler/file";
  import IconCode from "~icons/tabler/file-code";
  import IconMarkdown from "~icons/tabler/markdown";
  import IconPhoto from "~icons/tabler/photo";
  import IconLink from "~icons/tabler/link";
  import IconLoader from "~icons/tabler/loader-2";
  import IconDots from "~icons/tabler/dots";
  import type { FileTarget } from "./CodeFileActions.svelte";

  import { longPress } from "../../hooks/shared/long-press";
  import { makeRootFileTarget } from "../../domains/code/code-file-actions";
  import { navigateFileTreeKey } from "../../domains/code/file-tree-keyboard";
  import { xrTreePanel } from "../../domains/code/xr-code-navigation";
  import { openFileTreeMenu } from "../../domains/code/file-tree-menu";

  let {
    listing,
    rootPath,
    pending,
    activePath,
    directory,
    refresh,
    collapse,
    fetchDirectory,
    openFile,
    loadMore,
    menu,
    restore,
    dropPath = null,
  }: {
    listing: CodeListing | undefined;
    rootPath: string;
    pending: boolean;
    activePath: string;
    directory: string;
    refresh: number;
    collapse: number;
    fetchDirectory: (
      path: string,
      cursor: string,
      signal: AbortSignal,
    ) => Promise<CodeListing>;
    openFile: (path: string, id: string) => void;
    loadMore: () => void | Promise<void>;
    /** Opens the file actions menu for node, anchored at a point or control. */
    menu?: (
      node: FileTarget,
      at: HTMLElement | { x: number; y: number },
      row: HTMLElement,
    ) => void;
    /** Applied at mount: the listings are shown without refetching them. */
    restore?: CodeFileTreeState;
    /** The folder highlighted as the target of a file drag; "" is the root. */
    dropPath?: string | null;
  } = $props();
  const rootNode = $derived(makeRootFileTarget(rootPath));
  function contextMenu(event: MouseEvent, node: FileTarget) {
    openFileTreeMenu(event, node, menu);
  }
  // Rows register their node; one long-press listener on the tree resolves
  // the held row, so a directory refresh mid-press cannot cancel it.
  const rowNodes = new WeakMap<HTMLElement, FileTarget>();
  const register = (node: FileTarget) => (element: HTMLElement) => {
    rowNodes.set(element, node);
  };
  const hold = longPress((element) => {
    const node = rowNodes.get(element);
    if (node) menu?.(node, element, element);
  }, "button[data-file-entry]");
  // Rows mark the folder that receives files dropped on them: a folder row
  // its own folder, a file row its parent, the root row the root.
  const dropZones = new WeakMap<Element, FileTarget>();
  const dropZone = (node: FileTarget) => (element: HTMLElement) => {
    dropZones.set(element, node);
  };
  /** The folder receiving files dropped on element; the root by default. */
  export function dropTarget(element: Element | null): FileTarget {
    const zone = element?.closest("[data-code-drop]");
    return (zone && dropZones.get(zone)) ?? rootNode;
  }
  const restored = untrack(() => restore);
  let rootOpen = $state(restored?.rootOpen ?? true);
  const expanded = new SvelteSet<string>(restored?.expanded);
  const branches = new SvelteMap<string, CodeListing>(restored?.branches);
  const errors = new SvelteMap<string, string>();
  const loading = new SvelteSet<string>();
  const requests = new SvelteMap<string, AbortController>();
  const inflight = new SvelteMap<string, Promise<void>>();
  let revealSequence = 0;
  export function capture(): CodeFileTreeState {
    return {
      rootOpen,
      expanded: [...expanded],
      branches: [...branches],
    };
  }
  const xrNavigation = $derived(
    xrTreePanel({
      listing,
      expanded,
      branches,
      loading,
      errors,
      toggle,
      open: openFile,
      more: (path) => {
        if (path) void loadBranch(path, true);
        else void loadMore();
      },
    }),
  );
  export function xrPanel() {
    return xrNavigation;
  }
  function loadBranch(path: string, append = false): Promise<void> {
    const current = inflight.get(path);
    if (current) return current;
    const promise = readBranch(path, append);
    inflight.set(path, promise);
    void promise.finally(() => {
      if (inflight.get(path) === promise) inflight.delete(path);
    });
    return promise;
  }
  async function readBranch(path: string, append: boolean) {
    if (requests.has(path)) return;
    const request = new AbortController();
    requests.set(path, request);
    loading.add(path);
    errors.delete(path);
    try {
      const previous = branches.get(path);
      const result = await fetchDirectory(
        path,
        append ? (previous?.nextCursor ?? "") : "",
        request.signal,
      );
      if (!request.signal.aborted)
        branches.set(
          path,
          append && previous
            ? {
                ...result,
                children: [...previous.children, ...result.children],
              }
            : result,
        );
    } catch (error) {
      if (!request.signal.aborted) errors.set(path, errorMessage(error));
    } finally {
      if (requests.get(path) === request) {
        requests.delete(path);
        loading.delete(path);
      }
    }
  }
  function toggle(path: string, open: boolean) {
    if (open) {
      expanded.add(path);
      if (!branches.has(path)) void loadBranch(path);
    } else expanded.delete(path);
  }
  function abortRequests() {
    revealSequence++;
    inflight.clear();
    for (const request of requests.values()) request.abort();
    requests.clear();
    loading.clear();
  }
  onDestroy(abortRequests);
  // Mount is not a refresh or a collapse: restored folders stay as they were.
  let lastRefresh = untrack(() => refresh);
  let lastCollapse = untrack(() => collapse);
  for (const path of restored?.expanded ?? [])
    if (!branches.has(path)) void loadBranch(path);
  $effect(() => {
    if (refresh === lastRefresh) return;
    lastRefresh = refresh;
    untrack(() => {
      abortRequests();
      branches.clear();
      errors.clear();
      for (const path of expanded) void loadBranch(path);
    });
  });
  $effect(() => {
    if (collapse === lastCollapse) return;
    lastCollapse = collapse;
    untrack(() => {
      revealSequence++;
      expanded.clear();
      rootOpen = true;
    });
  });
  async function reveal(path: string) {
    const sequence = ++revealSequence;
    rootOpen = true;
    if (!path) return;
    const parts = path.split("/");
    for (let i = 1; i <= parts.length; i++) {
      const parent = parts.slice(0, i - 1).join("/");
      const child = parts.slice(0, i).join("/");
      const current = () => (parent ? branches.get(parent) : listing);
      while (!current()?.children.some((node) => node.path === child)) {
        const cursor = current()?.nextCursor;
        if (!cursor || sequence !== revealSequence) return;
        if (parent) await loadBranch(parent, true);
        else await loadMore();
        await tick();
        if (sequence !== revealSequence || current()?.nextCursor === cursor)
          return;
      }
      if (sequence !== revealSequence) return;
      expanded.add(child);
      if (!branches.has(child)) await loadBranch(child);
    }
  }
  // A restored tree already shows the folders the user had opened.
  let revealed = !!restored;
  $effect(() => {
    const path = directory;
    if (refresh !== lastRefresh) return;
    if (revealed) revealed = false;
    else untrack(() => void reveal(path));
  });
  const keyboard = navigateFileTreeKey;
</script>

{#snippet actions(node: FileTarget)}
  {#if menu}<ListButton
      variant="file-tree-action"
      tabindex={-1}
      aria-label={m.s5_file_actions_for({ name: node.name })}
      title={m.s5_more_actions()}
      onclick={(event) => {
        const button = event.currentTarget;
        const row =
          button.parentElement?.querySelector<HTMLElement>(
            "[data-file-entry]",
          ) ?? button;
        menu(node, button, row);
      }}><IconDots aria-hidden="true" /></ListButton
    >{/if}
{/snippet}

{#snippet contents(
  value: CodeListing | undefined,
  path: string,
  parent: FileTarget,
)}
  <ul class="m-0 list-none p-0">
    {#each value?.children ?? [] as node (node.nodeId)}
      {@const folder = node.kind === "directory"}
      <li>
        {#if folder}
          <Collapsible.Root
            open={expanded.has(node.path)}
            onOpenChange={(open) => toggle(node.path, open)}
          >
            <div
              class="group/row relative"
              data-code-drop
              {@attach dropZone(node)}
            >
              <Collapsible.Trigger>
                {#snippet child({ props })}
                  <ListButton
                    {...props}
                    variant="file-tree"
                    data-file-entry
                    data-drop-target={dropPath === node.path || undefined}
                    onkeydown={keyboard}
                    oncontextmenu={(event) => contextMenu(event, node)}
                    {@attach register(node)}
                    title={node.path}
                  >
                    {#if loading.has(node.path)}<IconLoader
                        class="size-3.5 shrink-0 animate-spin"
                        aria-hidden="true"
                      />{:else if expanded.has(node.path)}<IconChevronDown
                        class="size-3.5 shrink-0"
                        aria-hidden="true"
                      />{:else}<IconChevronRight
                        class="size-3.5 shrink-0"
                        aria-hidden="true"
                      />{/if}
                    {#if expanded.has(node.path)}<IconFolderOpen
                        class="size-4 shrink-0 text-muted-foreground"
                        aria-hidden="true"
                      />{:else}<IconFolder
                        class="size-4 shrink-0 text-muted-foreground"
                        aria-hidden="true"
                      />{/if}
                    <span class="truncate">{node.name}</span>
                  </ListButton>
                {/snippet}
              </Collapsible.Trigger>
              {@render actions(node)}
            </div>
            <Collapsible.Content>
              <div class="ml-3 border-l pl-1">
                {@render contents(branches.get(node.path), node.path, node)}
              </div>
            </Collapsible.Content>
          </Collapsible.Root>
        {:else}
          {@const disabled = node.kind !== "file"}
          {@const FileIcon = node.mediaType.startsWith("image/")
            ? IconPhoto
            : node.kind === "symlink"
              ? IconLink
              : node.name.endsWith(".md")
                ? IconMarkdown
                : /\.(go|ts|js|svelte|json|yaml|toml|css|html)$/.test(node.name)
                  ? IconCode
                  : IconFile}
          <div
            class="group/row relative"
            data-code-drop
            {@attach dropZone(parent)}
          >
            <ListButton
              variant="file-tree"
              data-file-entry
              onkeydown={keyboard}
              oncontextmenu={(event) => contextMenu(event, node)}
              {@attach register(node)}
              aria-label={node.name}
              aria-current={activePath === node.path ? "page" : undefined}
              {disabled}
              title={`${node.path}${node.linkTarget ? ` → ${node.linkTarget}` : ""}${disabled ? ` · ${m.s5_unsupported_open_path()}` : ""}`}
              onclick={() => openFile(node.path, node.nodeId)}
            >
              <span class="size-3.5 shrink-0"></span>
              <FileIcon
                class="size-4 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              <span class="truncate">{node.name}</span>
              {#if node.gitStatus}<span
                  class="ml-auto shrink-0 text-warning"
                  aria-label={m.s5_git_status_aria({ status: node.gitStatus })}
                  >{node.gitStatus}</span
                >{/if}
              {#if node.ignored}<span
                  class="ml-auto shrink-0 text-muted-foreground"
                  >{m.s5_ignored_badge()}</span
                >{/if}
              {#if disabled}<span class="ml-auto shrink-0 text-muted-foreground"
                  >{node.kind === "boundary"
                    ? m.s5_node_boundary()
                    : node.kind === "symlink"
                      ? m.s5_node_symlink()
                      : m.s5_node_special()}</span
                >{/if}
            </ListButton>
            {@render actions(node)}
          </div>
        {/if}
      </li>
    {/each}
  </ul>
  {#if errors.has(path)}
    <p role="alert" class="px-2 py-1 text-xs break-all text-destructive">
      {errors.get(path)}
    </p>
    <Button variant="ghost" size="xs" onclick={() => void loadBranch(path)}
      >{m.s5_retry_load_dir()}</Button
    >
  {:else if loading.has(path) || (!path && pending)}
    <p role="status" class="px-2 py-1 text-xs text-muted-foreground">
      {m.s5_reading_dir()}
    </p>
  {:else if value?.children.length === 0}<p
      class="px-2 py-1 text-xs text-muted-foreground"
    >
      {m.s5_empty_dir()}
    </p>{/if}
  {#if value?.nextCursor}<Button
      variant="ghost"
      size="xs"
      disabled={path ? loading.has(path) : pending}
      onclick={() => (path ? void loadBranch(path, true) : loadMore())}
      >{m.s5_next_page()}</Button
    >{/if}
  {#if value?.limited || value?.complete === false}<p
      role="status"
      class="px-2 py-1 text-xs text-muted-foreground"
    >
      {m.s5_results_limited_narrow_path()}
    </p>{/if}
{/snippet}

<ScrollArea class="min-h-0 flex-1">
  <div
    class="p-1"
    role="group"
    aria-label={m.s5_file_tree()}
    data-file-tree
    {@attach hold}
  >
    <Collapsible.Root bind:open={rootOpen}>
      <div
        class="group/row relative"
        data-code-drop
        {@attach dropZone(rootNode)}
      >
        <Collapsible.Trigger>
          {#snippet child({ props })}
            <ListButton
              {...props}
              variant="file-tree-root"
              data-file-entry
              data-drop-target={dropPath === "" || undefined}
              onkeydown={keyboard}
              oncontextmenu={(event) => contextMenu(event, rootNode)}
              {@attach register(rootNode)}
              title={rootPath}
            >
              {#if rootOpen}<IconChevronDown
                  class="size-3.5 shrink-0"
                  aria-hidden="true"
                />{:else}<IconChevronRight
                  class="size-3.5 shrink-0"
                  aria-hidden="true"
                />{/if}
              <span class="truncate">{rootNode.name}</span>
            </ListButton>
          {/snippet}
        </Collapsible.Trigger>
        {@render actions(rootNode)}
      </div>
      <Collapsible.Content>
        {@render contents(listing, "", rootNode)}
      </Collapsible.Content>
    </Collapsible.Root>
  </div>
</ScrollArea>
