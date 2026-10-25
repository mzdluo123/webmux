<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { SvelteMap, SvelteSet } from "svelte/reactivity";
  import type { CodeListing } from "../schemas";
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
    loadMore: () => void;
  } = $props();
  let rootOpen = $state(true);
  const expanded = new SvelteSet<string>();
  const branches = new SvelteMap<string, CodeListing>();
  const errors = new SvelteMap<string, string>();
  const loading = new SvelteSet<string>();
  const requests = new Map<string, AbortController>();
  async function loadBranch(path: string, append = false) {
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
      if (!request.signal.aborted) errors.set(path, (error as Error).message);
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
    for (const request of requests.values()) request.abort();
    requests.clear();
    loading.clear();
  }
  onDestroy(abortRequests);
  let lastRefresh = -1;
  let lastCollapse = -1;
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
      expanded.clear();
      rootOpen = true;
    });
  });
  $effect(() => {
    const path = directory;
    if (!path) return;
    untrack(() => {
      rootOpen = true;
      const parts = path.split("/");
      for (let i = 1; i <= parts.length; i++)
        toggle(parts.slice(0, i).join("/"), true);
    });
  });
  function keyboard(event: KeyboardEvent) {
    const target = (event.target as HTMLElement).closest<HTMLButtonElement>(
      "button[data-file-entry]",
    );
    if (!target) return;
    const rows = [
      ...(event.currentTarget as HTMLElement)
        .closest("[data-file-tree]")!
        .querySelectorAll<HTMLButtonElement>("button[data-file-entry]"),
    ].filter((row) => !row.disabled && row.getClientRects().length > 0);
    const index = rows.indexOf(target);
    let next: HTMLButtonElement | undefined;
    if (event.key === "ArrowDown")
      next = rows[Math.min(index + 1, rows.length - 1)];
    else if (event.key === "ArrowUp") next = rows[Math.max(index - 1, 0)];
    else if (event.key === "Home") next = rows[0];
    else if (event.key === "End") next = rows.at(-1);
    else if (event.key === "ArrowRight") {
      if (target.getAttribute("aria-expanded") === "false") target.click();
      else if (target.getAttribute("aria-expanded") === "true")
        next = rows[index + 1];
    } else if (event.key === "ArrowLeft") {
      if (target.getAttribute("aria-expanded") === "true") target.click();
      else
        next =
          target
            .closest("li")
            ?.parentElement?.closest("li")
            ?.querySelector<HTMLButtonElement>("button[data-file-entry]") ??
          rows[0];
    } else return;
    event.preventDefault();
    next?.focus();
  }
</script>

{#snippet contents(value: CodeListing | undefined, path: string)}
  <ul class="m-0 list-none p-0">
    {#each value?.children ?? [] as node (node.nodeId)}
      {@const folder = node.kind === "directory"}
      <li>
        {#if folder}
          <Collapsible.Root
            open={expanded.has(node.path)}
            onOpenChange={(open) => toggle(node.path, open)}
          >
            <Collapsible.Trigger>
              {#snippet child({ props })}
                <ListButton
                  {...props}
                  variant="file-tree"
                  data-file-entry
                  onkeydown={keyboard}
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
            <Collapsible.Content>
              <div class="ml-3 border-l pl-1">
                {@render contents(branches.get(node.path), node.path)}
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
          <ListButton
            variant="file-tree"
            data-file-entry
            onkeydown={keyboard}
            aria-label={node.name}
            aria-current={activePath === node.path ? "page" : undefined}
            {disabled}
            title={`${node.path}${node.linkTarget ? ` → ${node.linkTarget}` : ""}${disabled ? " · 不支持打开此路径" : ""}`}
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
                aria-label={`Git 状态 ${node.gitStatus}`}>{node.gitStatus}</span
              >{/if}
            {#if node.ignored}<span
                class="ml-auto shrink-0 text-muted-foreground">忽略</span
              >{/if}
            {#if disabled}<span class="ml-auto shrink-0 text-muted-foreground"
                >{node.kind === "boundary"
                  ? "边界"
                  : node.kind === "symlink"
                    ? "链接"
                    : "特殊"}</span
              >{/if}
          </ListButton>
        {/if}
      </li>
    {/each}
  </ul>
  {#if errors.has(path)}
    <p role="alert" class="px-2 py-1 text-xs break-all text-destructive">
      {errors.get(path)}
    </p>
    <Button variant="ghost" size="xs" onclick={() => void loadBranch(path)}
      >重试加载目录</Button
    >
  {:else if loading.has(path) || (!path && pending)}
    <p role="status" class="px-2 py-1 text-xs text-muted-foreground">
      读取目录…
    </p>
  {:else if value?.children.length === 0}<p
      class="px-2 py-1 text-xs text-muted-foreground"
    >
      空目录
    </p>{/if}
  {#if value?.nextCursor}<Button
      variant="ghost"
      size="xs"
      disabled={path ? loading.has(path) : pending}
      onclick={() => (path ? void loadBranch(path, true) : loadMore())}
      >下一页</Button
    >{/if}
  {#if value?.limited || value?.complete === false}<p
      role="status"
      class="px-2 py-1 text-xs text-muted-foreground"
    >
      结果受限，请收窄路径；未列出全部文件。
    </p>{/if}
{/snippet}

<ScrollArea class="min-h-0 flex-1">
  <div class="p-1" role="group" aria-label="文件树" data-file-tree>
    <Collapsible.Root bind:open={rootOpen}>
      <Collapsible.Trigger>
        {#snippet child({ props })}
          <ListButton
            {...props}
            variant="file-tree-root"
            data-file-entry
            onkeydown={keyboard}
            title={rootPath}
          >
            {#if rootOpen}<IconChevronDown
                class="size-3.5 shrink-0"
                aria-hidden="true"
              />{:else}<IconChevronRight
                class="size-3.5 shrink-0"
                aria-hidden="true"
              />{/if}
            <span class="truncate"
              >{rootPath.split("/").filter(Boolean).at(-1) ?? "根目录"}</span
            >
          </ListButton>
        {/snippet}
      </Collapsible.Trigger>
      <Collapsible.Content>
        {@render contents(listing, "")}
      </Collapsible.Content>
    </Collapsible.Root>
  </div>
</ScrollArea>
