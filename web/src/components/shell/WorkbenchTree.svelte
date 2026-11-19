<script lang="ts" module>
  export {
    workbenchTree,
    type TreeNode,
  } from "../../domains/workspace/workbench-tree";
</script>

<script lang="ts">
  import { tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    ArchivedSession,
    HomeWorktrees,
    Repository,
    SessionInfo,
    WorktreePins,
  } from "../../contracts/schemas";
  import {
    homeLayout,
    setPinned,
  } from "../../domains/workspace/home-worktrees";
  import { homeWorktrees } from "../../domains/workspace/home-worktrees.svelte";
  import { worktreePins } from "../../domains/workspace/worktree-pins.svelte";
  import { rowStatus } from "../../domains/sessions/session-status";
  import { resumesOnOpen } from "../../domains/sessions/archived";
  import {
    defaultNodeOpen,
    readFolderMemory,
    reconcileFolderMemory,
    workbenchTree,
    type FolderMemory,
    type TreeNode,
  } from "../../domains/workspace/workbench-tree";
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconFolder from "~icons/tabler/folder";
  import IconFolderOff from "~icons/tabler/folder-off";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { store, stored } from "../../platform/api";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import LaunchConfigIcon from "../settings/LaunchConfigIcon.svelte";
  import RowStatus from "../shared/RowStatus.svelte";
  import { underRowAction } from "../../hooks/shared/sidebar-row";
  import { switchableWorktree } from "../../domains/workspace/worktree-sync";
  import MainWorktreeMark from "../workspace/MainWorktreeMark.svelte";
  import PinnedWorktreeMark from "../workspace/PinnedWorktreeMark.svelte";
  import UpstreamStatus from "../workspace/UpstreamStatus.svelte";
  import SidebarProjectAvatar from "./SidebarProjectAvatar.svelte";
  import SidebarRepositoryMenu from "./SidebarRepositoryMenu.svelte";
  import SidebarRowMenu from "./SidebarRowMenu.svelte";
  import SidebarSessionMenu from "./SidebarSessionMenu.svelte";
  import {
    notableUpstream,
    treeNodeAriaLabel,
    treeNodeStateText,
    treeNodeTitle,
    treeWorktreePinMenuProps,
  } from "./workbench-tree-labels";
  import WorktreeMenuItems from "../workspace/WorktreeMenuItems.svelte";

  // Arrow keys move between rows, with the row menu as the next tab stop.
  let {
    repositories,
    sessions,
    records,
    resuming = "",
    home,
    pins = worktreePins.value,
    scope,
    worktree,
    current,
    canCreate,
    configIcons,
    onscope,
    onopen,
    onnewworktree,
    onhomeworktrees,
    onmanageworktrees,
    onrepositorydetails,
    onrepositorysettings,
    onworktreedetails,
    onremoveworktree,
    onswitchbranch,
    ontoggletoppin,
    onmovetoppin,
    onbranchoperation,
    onopengit,
    onrename,
    onclose,
    onresume,
    onremove,
  }: {
    repositories: readonly Repository[];
    sessions: readonly SessionInfo[];
    records: readonly ArchivedSession[];
    resuming?: string;
    home: HomeWorktrees;
    pins?: WorktreePins;
    /** The home view shown ("all", "direct" or a repository ID); "" elsewhere. */
    scope: string;
    worktree: string;
    current: string;
    canCreate: boolean;
    configIcons: ReadonlyMap<string, string>;
    onscope: (scope: string, worktreeId?: string) => void;
    onopen: (session: SessionInfo) => void;
    onnewworktree: (repositoryId: string) => void;
    onhomeworktrees: (repositoryId: string) => void;
    onmanageworktrees: (repositoryId: string) => void;
    onrepositorydetails: (repositoryId: string) => void;
    onrepositorysettings?: (repositoryId: string) => void;
    onworktreedetails: (repositoryId: string, worktreeId: string) => void;
    onremoveworktree: (repositoryId: string, worktreeId: string) => void;
    onswitchbranch: (repositoryId: string, worktreeId: string) => void;
    ontoggletoppin?: (repositoryId: string, worktreeId: string) => void;
    onmovetoppin?: (
      repositoryId: string,
      worktreeId: string,
      offset: -1 | 1,
    ) => void;
    onbranchoperation?: (
      repositoryId: string,
      operation: BranchOperation,
    ) => void;
    onopengit?: (
      repositoryId: string,
      worktreeId?: string,
      view?: "commit",
    ) => void;
    onrename: (session: SessionInfo) => void;
    onclose: (session: SessionInfo) => void;
    onresume: (record: ArchivedSession) => void;
    onremove: (record: ArchivedSession) => void;
  } = $props();

  const fullTree = $derived(
    workbenchTree(repositories, sessions, home, records, pins),
  );
  let saved = $state<FolderMemory>(
    readFolderMemory(stored("workbenchTree", "{}")),
  );
  const effective = $derived(reconcileFolderMemory(saved, fullTree));
  $effect(() => {
    const next = effective;
    if (next !== saved) {
      saved = next;
      store("workbenchTree", JSON.stringify(next));
    }
  });
  // The current session's repository and worktree open without being saved.
  const forced = $derived.by(() => {
    const find = (list: TreeNode[], path: string[]): string[] => {
      for (const node of list) {
        if (
          current &&
          (node.session?.id === current || node.record?.id === current)
        )
          return path;
        const found = find(node.children, [...path, node.key]);
        if (found.length) return found;
      }
      return [];
    };
    return find(fullTree, []);
  });
  function isOpen(node: TreeNode) {
    if (!node.children.length || node.reason) return false;
    if (forced.includes(node.key)) return true;
    // Default folds reveal live and waiting-to-resume sessions.
    return effective.open[node.key] ?? defaultNodeOpen(node);
  }
  function setOpen(node: TreeNode, open: boolean) {
    saved = {
      open: { ...effective.open, [node.key]: open },
      sessions: effective.sessions,
    };
    store("workbenchTree", JSON.stringify(saved));
  }
  const visible = $derived.by(() => {
    const rows: TreeNode[] = [];
    const walk = (list: TreeNode[]) => {
      for (const node of list) {
        rows.push(node);
        if (isOpen(node)) walk(node.children);
      }
    };
    walk(fullTree);
    return rows;
  });
  let focusKey = $state("all");
  const tabStop = $derived(
    visible.some((node) => node.key === focusKey)
      ? focusKey
      : (visible.find((node) => isCurrent(node))?.key ?? "all"),
  );
  let root = $state<HTMLElement>();
  async function focusRow(key: string) {
    focusKey = key;
    await tick();
    root
      ?.querySelector<HTMLElement>(`[data-tree-key="${CSS.escape(key)}"]`)
      ?.focus();
  }
  function keydown(event: KeyboardEvent, node: TreeNode) {
    const index = visible.findIndex((row) => row.key === node.key);
    let next: TreeNode | undefined;
    switch (event.key) {
      case "ArrowDown":
        next = visible[index + 1];
        break;
      case "ArrowUp":
        next = visible[index - 1];
        break;
      case "Home":
        next = visible[0];
        break;
      case "End":
        next = visible.at(-1);
        break;
      case "ArrowRight":
        if (node.children.length && !node.reason) {
          if (!isOpen(node)) setOpen(node, true);
          else next = node.children[0];
        }
        break;
      case "ArrowLeft":
        if (isOpen(node)) setOpen(node, false);
        else next = visible.find((row) => row.key === node.parent);
        break;
      default:
        return;
    }
    event.preventDefault();
    if (next) void focusRow(next.key);
  }
  function isCurrent(node: TreeNode) {
    if (node.session || node.record)
      return (node.session?.id ?? node.record?.id) === current;
    if (current || node.kind === "missing") return false;
    if (node.kind === "worktree")
      return scope === node.repositoryId && worktree === node.worktreeId;
    return node.kind === "repository"
      ? scope === node.repositoryId && !worktree
      : scope === node.key;
  }
  function activate(node: TreeNode) {
    focusKey = node.key;
    if (node.session) onopen(node.session);
    else if (node.record) onresume(node.record);
    else if (node.kind === "worktree" || node.kind === "missing")
      onscope(node.repositoryId ?? "all", node.worktreeId);
    else
      onscope(
        node.kind === "repository" ? (node.repositoryId ?? "all") : node.key,
      );
  }
  function sessionIcon(session: SessionInfo) {
    return (
      session.agent?.icon ??
      (session.configId ? configIcons.get(session.configId) : undefined) ??
      ""
    );
  }
</script>

{#snippet counts(node: TreeNode, actioned: boolean, reserve: boolean)}
  <!-- Right-aligned, so counts end where session rows' status dots do; a
       row's "⋯" menu takes their place on hover. `reserve` keeps the slot
       empty so the menu never covers the marks before it. -->
  {#if node.waiting}<span
      class={[
        "w-12 shrink-0 truncate text-right text-xs text-warning",
        actioned && underRowAction,
      ]}
      title={m.s1_waiting_sessions_title({ count: node.waiting })}
      >{m.s1_waiting_count({ count: node.waiting })}</span
    >{:else if node.count}<span
      class={`w-12 shrink-0 text-right text-xs text-muted-foreground tabular-nums ${actioned ? underRowAction : ""}`}
      >{node.count}</span
    >{:else if reserve}<span
      class="w-12 shrink-0 any-pointer-coarse:hidden"
      aria-hidden="true"
    ></span>{/if}
{/snippet}

{#snippet chevron(node: TreeNode)}
  {#if node.kind !== "session" && node.children.length && !node.reason}
    {@const label = isOpen(node)
      ? m.s1_collapse_node({ label: node.label })
      : m.s1_expand_node({ label: node.label })}
    <!-- Keyboard users fold with the arrow keys, so this is not a tab stop. -->
    <Button
      variant="quiet"
      size="icon-xs"
      class="size-6 shrink-0"
      data-tree-toggle
      tabindex={-1}
      aria-label={label}
      title={label}
      onclick={() => setOpen(node, !isOpen(node))}
      ><IconChevronRight
        class={`transition-transform motion-reduce:transition-none ${isOpen(node) ? "rotate-90" : ""}`}
        aria-hidden="true"
      /></Button
    >
  {:else}<span class="size-6 shrink-0"></span>{/if}
{/snippet}

{#snippet row(node: TreeNode)}
  {@const status = node.session
    ? rowStatus(node.session)
    : node.record
      ? resumesOnOpen(node.record)
        ? "dormant"
        : "ended"
      : undefined}
  {@const stateText = treeNodeStateText(node, resuming, status)}
  {@const open = isOpen(node)}
  {@const upstream = notableUpstream(node)}
  {@const mainWorktree = node.worktree?.kind === "main"}
  {@const sessionId =
    node.session?.id ?? (status === "dormant" ? node.record?.id : undefined)}
  <li
    role="none"
    data-sidebar-session-id={sessionId}
    data-layout-row={sessionId ? undefined : node.key}
    class={`flex items-center ${node.level === 3 ? "pl-8" : node.level === 2 ? "pl-4" : ""}`}
  >
    {@render chevron(node)}
    <div class="group/row relative min-w-0 flex-1">
      <ListButton
        variant="file-tree-overlay"
        role="treeitem"
        aria-level={node.level}
        aria-expanded={node.children.length && !node.reason ? open : undefined}
        aria-selected={isCurrent(node)}
        aria-current={isCurrent(node) ? "page" : undefined}
        aria-label={treeNodeAriaLabel(node, stateText)}
        title={treeNodeTitle(node, status)}
        tabindex={tabStop === node.key ? 0 : -1}
        data-tree-key={node.key}
        onfocus={() => (focusKey = node.key)}
        onkeydown={(event: KeyboardEvent) => keydown(event, node)}
        aria-busy={node.record ? resuming === node.record.id : undefined}
        disabled={!!resuming && !!node.record}
        onclick={() => activate(node)}
      >
        {#if node.kind === "all"}<IconLayoutDashboard aria-hidden="true" />
        {:else if node.kind === "repository"}<SidebarProjectAvatar
            name={node.label}
            repositoryId={node.repositoryId}
          />
        {:else if node.kind === "direct"}<IconFolder aria-hidden="true" />
        {:else if node.kind === "worktree" || node.kind === "missing"}<IconGitBranch
            aria-hidden="true"
          />
        {:else if node.session || node.record}<LaunchConfigIcon
            icon={node.session
              ? sessionIcon(node.session)
              : node.record?.configId
                ? (configIcons.get(node.record.configId) ?? "")
                : ""}
            size="xs"
          />{/if}
        <span class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
          <span
            class={`min-w-0 truncate ${node.kind === "repository" ? "font-medium" : ""} ${node.reason || status === "ended" ? "text-muted-foreground" : ""}`}
            >{node.label}</span
          >
          {#if mainWorktree}<MainWorktreeMark />{/if}
          {#if node.pinned}<PinnedWorktreeMark />{/if}
        </span>
        {#if upstream}<span class="flex shrink-0 items-center gap-1"
            ><UpstreamStatus state={upstream} indicator /></span
          >{/if}
        {#if node.record?.released}<IconFolderOff
            class="text-muted-foreground"
            aria-hidden="true"
          />{/if}
        {#if node.reason}<IconAlertCircle
            class="text-warning"
            aria-hidden="true"
          />
        {:else if node.record && resuming === node.record.id}<span
            class="flex shrink-0 items-center gap-1 text-xs font-normal text-muted-foreground"
            ><IconLoader2
              class="size-3.5 animate-spin motion-reduce:animate-none"
              aria-hidden="true"
            />{m.s1_resuming_ellipsis()}</span
          >
        {:else if status}<RowStatus {status} />
        {:else}{@render counts(
            node,
            (node.kind === "repository" && !node.reason && canCreate) ||
              node.kind === "worktree",
            !!upstream || mainWorktree || !!node.pinned,
          )}{/if}
      </ListButton>
      {#if node.kind === "repository" && canCreate}
        <SidebarRepositoryMenu
          unavailable={!!node.reason}
          label={node.label}
          tabindex={tabStop === node.key ? 0 : -1}
          onnewworktree={() => onnewworktree(node.repositoryId ?? "")}
          onhome={() => onhomeworktrees(node.repositoryId ?? "")}
          onmanage={() => onmanageworktrees(node.repositoryId ?? "")}
          ondetails={() => onrepositorydetails(node.repositoryId ?? "")}
          onsettings={onrepositorysettings &&
            (() => onrepositorysettings(node.repositoryId ?? ""))}
          ongit={onopengit && (() => onopengit(node.repositoryId ?? ""))}
        />
      {:else if node.kind === "worktree" && node.repositoryId && node.worktreeId}
        {@const repositoryId = node.repositoryId}
        {@const worktreeId = node.worktreeId}
        {@const repo = repositories.find((repo) => repo.id === repositoryId)}
        {@const pinned =
          repo &&
          homeLayout(home, repo, []).shown.some(
            ({ tree }) => tree.id === worktreeId,
          )}
        <SidebarRowMenu
          ariaLabel={m.s1_worktree_actions({ label: node.label })}
          tabindex={tabStop === node.key ? 0 : -1}
        >
          <WorktreeMenuItems
            ondetails={() => onworktreedetails(repositoryId, worktreeId)}
            onswitch={node.worktree && switchableWorktree(node.worktree)
              ? () => onswitchbranch(repositoryId, worktreeId)
              : undefined}
            onremove={node.removable
              ? () => onremoveworktree(repositoryId, worktreeId)
              : undefined}
            worktree={node.worktree}
            {pinned}
            ontogglepin={repo &&
              (() =>
                homeWorktrees.save(
                  setPinned(home, repo, [worktreeId], !pinned),
                ))}
            {...treeWorktreePinMenuProps({
              repositories,
              sessions,
              home,
              pins,
              repositoryId,
              worktreeId,
              ontoggle: ontoggletoppin,
              onmove: onmovetoppin,
            })}
            onoperation={onbranchoperation &&
              ((operation) => onbranchoperation(repositoryId, operation))}
            ongit={onopengit && (() => onopengit(repositoryId, worktreeId))}
            oncommit={onopengit &&
              (() => onopengit(repositoryId, worktreeId, "commit"))}
          />
        </SidebarRowMenu>
      {:else if node.session}
        <SidebarSessionMenu
          label={node.label}
          tabindex={tabStop === node.key ? 0 : -1}
          target={{ kind: "live", session: node.session, onrename, onclose }}
        />
      {:else if node.record}
        {@const record = node.record}
        <SidebarSessionMenu
          label={node.label}
          tabindex={tabStop === node.key ? 0 : -1}
          disabled={!!resuming}
          target={{
            kind: "archived",
            onresume: () => onresume(record),
            onremove: () => onremove(record),
          }}
        />
      {/if}
    </div>
  </li>
  {#if open}
    <li role="none">
      <ul role="group" class="space-y-0.5">
        {#each node.children as child (child.key)}{@render row(child)}{/each}
      </ul>
    </li>
  {/if}
{/snippet}

<ul
  bind:this={root}
  role="tree"
  aria-label={m.s1_repositories_and_sessions()}
  class="space-y-0.5 text-sm"
>
  {#each fullTree as node (node.key)}{@render row(node)}{/each}
</ul>
