<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import { fileTabLabels } from "../../domains/code/file-tab-labels";
  import CodeNavigationPanel from "./CodeNavigationPanel.svelte";
  import CodeReaderGroup from "./CodeReaderGroup.svelte";
  import { getDock } from "../shared/dock-context";
  import CodeWorkspaceSplit from "./CodeWorkspaceSplit.svelte";
  import { onMount, untrack } from "svelte";
  import { SvelteMap } from "svelte/reactivity";
  import CodeNavigator from "./CodeNavigator.svelte";
  import CodeWorkspaceHeader from "./CodeWorkspaceHeader.svelte";
  import { api } from "../../platform/api";
  import {
    CodeFileSchema,
    CodeChangesSchema,
    CodeDiffSchema,
    CodeLocationSchema,
    type CodeChange,
    type CodeReveal,
    type CodeWorkspaceSnapshot,
  } from "../../contracts/schemas";
  import type { z } from "zod";
  import type { CodeHost } from "../../domains/code/code-host";
  import { nodeId } from "../../domains/code/code-reference";
  import {
    createMarkdownWorkspace,
    locateRevealTarget,
  } from "../../domains/code/code-reveal";
  import {
    shouldCloseOverlayNav,
    type CodeTabOrigin,
    type CodeWorkspaceTab,
  } from "../../domains/code/code-workspace";
  import { useCodeWorkspaceState } from "../../hooks/code/code-workspace-state.svelte";
  import { useCodeReaderDock } from "../../hooks/code/code-reader-dock.svelte";
  import * as Tabs from "$lib/components/ui/tabs";
  import CodeDocumentView from "./CodeDocumentView.svelte";
  import "../../fonts-terminal.css";

  let {
    id,
    visible,
    host,
    initial,
    popout,
    onOpenGit,
    onInspectGitFile,
    docked,
    notice = "",
    inert = false,
    focusRestored = false,
  }: {
    id: string;
    visible: boolean;
    host: CodeHost;
    /** Reading state captured by another instance, such as another window. */
    initial?: CodeWorkspaceSnapshot;
    /** The pop-out toggle; absent where the panel cannot change windows. */
    popout?: { popped: boolean; toggle: () => void };
    onOpenGit?: () => void;
    onInspectGitFile?: (
      path: string,
      tab: "history" | "blame" | "compare",
    ) => void;
    docked?: { files: string; reader: string };
    notice?: string;
    inert?: boolean;
    focusRestored?: boolean;
  } = $props();
  const seed = untrack(() => initial);
  const dock = untrack(() => (docked ? getDock() : undefined));
  let section = $state<HTMLElement>();
  let navigationHost = $state<HTMLDivElement>();
  let navigator = $state<CodeNavigator>();
  let dropStatus = $state("");
  const views = new SvelteMap<string, CodeDocumentView>();
  const readingAreas = new SvelteMap<string, HTMLElement>();
  const shown = $derived(visible || !!popout?.popped);
  let base = $derived(`/sessions/${id}/code`);
  const readers = useCodeReaderDock(
    seed,
    () => id,
    untrack(() => docked),
    dock,
  );
  const ws = useCodeWorkspaceState({
    id: () => id,
    base: () => base,
    shown: () => shown,
    seed,
    onOpen: (key, added, activate) => {
      readers.open(key, added, activate);
      if (!docked && activate && shouldCloseOverlayNav(section))
        ws.s.navOpen = false;
    },
    onClose: (key) => readers.close(key, ws.s.active),
  });
  const activeTab = $derived(ws.s.tabs.find((tab) => tab.key === ws.s.active));
  const activePath = $derived(activeTab?.file?.document.path ?? "");
  const labels = $derived(fileTabLabels(ws.s.tabs));
  const filesShown = $derived(!!docked && !!dock?.rect(docked.files));
  const visibleDocument = (tab: CodeWorkspaceTab) => {
    const panel = readers.panel(tab.key);
    return !!panel && !!dock?.rect(panel);
  };
  const alertTarget = $derived(
    (activeTab && visibleDocument(activeTab) ? activeTab : undefined) ??
      ws.s.tabs.find(visibleDocument),
  );
  function activateReader(key: string) {
    readers.select(key);
    if (ws.s.active !== key) ws.activate(key);
  }
  function showRevealError() {
    if (docked && !shown) dock?.show(docked.files);
  }
  function openRight(path: string, opaqueId: string) {
    const origin = ws.s.context;
    if (origin)
      void openFileInOrigin(origin, path, opaqueId, readers.rightAnchor());
  }

  /** The reading state of this workspace, for another instance to restore. */
  export function capture(): CodeWorkspaceSnapshot {
    const snapshot = ws.snapshotState(
      (tab) => views.get(tab.key)?.capture() ?? $state.snapshot(tab.restore),
      () => navigator?.capture() ?? seed?.navigator,
    );
    return { ...snapshot, readerGroups: readers.capture() };
  }
  export function bounds() {
    const area = readingAreas.get(ws.s.active);
    return (
      area?.clientWidth ? area : (section ?? navigationHost)
    )?.getBoundingClientRect();
  }
  async function locate(input: string) {
    if (!ws.s.context) return;
    await show(
      await api(
        `${base}/locate?${ws.params({ path: input })}`,
        CodeLocationSchema,
      ),
    );
  }
  async function show(target: z.infer<typeof CodeLocationSchema>) {
    if (target.kind === "file") return openFile(target.path, target.nodeId);
    ws.s.nav = "files";
    ws.s.navOpen = true;
    if (docked) dock?.show(docked.files);
    ws.s.directory = "";
    ws.s.query = "";
    await ws.load();
    ws.s.directory = target.path;
    return "";
  }
  async function openFileInOrigin(
    origin: CodeTabOrigin,
    path: string,
    opaqueId = nodeId(path),
    rightOf?: string,
  ) {
    const place = rightOf
      ? (key: string, added: boolean, activate: boolean) =>
          readers.open(key, added, activate, activate ? rightOf : undefined)
      : undefined;
    return ws.openTab(
      `${origin.contextId}:${opaqueId}:worktree`,
      `${base}/file?${ws.params({ contextId: origin.contextId, nodeId: opaqueId })}`,
      CodeFileSchema,
      (value, activate) =>
        ws.install(origin, value, undefined, activate, place),
      place,
    );
  }
  async function openFile(path: string, opaqueId = nodeId(path)) {
    const origin = ws.s.context;
    if (!origin) return "";
    return openFileInOrigin(origin, path, opaqueId);
  }
  let headingSequence = 0;
  let jumpSequence = 0;
  const markdownWorkspace = createMarkdownWorkspace({
    id: () => id,
    base: () => base,
    getContext: () => ws.s.context,
    params: (extra) => ws.params(extra),
    show,
    getTabs: () => ws.s.tabs,
    nextHeadingKey: () => ++headingSequence,
  });
  async function openDiff(change: CodeChange, origin = ws.s.context) {
    if (!origin) return;
    return ws.openTab(
      `${origin.contextId}:${change.nodeId}:diff:${change.group}`,
      `${base}/diff?${ws.params({ contextId: origin.contextId, changeId: change.changeId })}`,
      CodeDiffSchema,
      (value, activate) => ws.install(origin, undefined, value, activate),
    );
  }
  onMount(() => {
    readers.restore(focusRestored);
    return ws.mountWorkspace();
  });
  ws.syncConnectionCheck(() => host.connection);

  let revealed = untrack(() => (seed ? (host.reveal?.key ?? 0) : 0));
  $effect(() => {
    const target = host.reveal;
    const current = ws.s.context;
    if (!target || target.key === revealed) return;
    if (untrack(() => ws.s.root) !== "workspace") {
      untrack(() => ws.switchRoot("workspace"));
      return;
    }
    if (!current) {
      if (ws.s.error) untrack(showRevealError);
      return;
    }
    revealed = target.key;
    untrack(() => void showReveal(target));
  });
  async function showReveal(target: CodeReveal) {
    const current = ws.s.context;
    if (!current) return;
    try {
      const { origin, location, usedRawPath } = await locateRevealTarget(
        base,
        (extra) => ws.params(extra),
        current,
        target,
      );
      if (location.kind !== "file") {
        if (origin.contextId === ws.s.context?.contextId) await show(location);
        return;
      }
      const mode =
        target.mode ??
        (target.line !== undefined || target.heading ? "file" : "diff");
      if (mode === "diff" && origin.diffAvailable) {
        const found = await api(
          `${base}/changes?${ws.params({ contextId: origin.contextId, nodeId: location.nodeId })}`,
          CodeChangesSchema,
        );
        const change =
          found.entries.find((entry) => entry.group !== "staged") ??
          found.entries[0];
        if (change) {
          if (await openDiff(change, origin)) showRevealError();
          return;
        }
      }
      const failure = await openFileInOrigin(
        origin,
        location.path,
        location.nodeId,
      );
      if (failure) {
        showRevealError();
        return;
      }
      if (mode !== "file") return;
      const tab = ws.s.tabs.find(
        (t) => t.key === `${origin.contextId}:${location.nodeId}:worktree`,
      );
      if (!tab) return;
      if (!usedRawPath && target.line !== undefined) {
        tab.jump = {
          line: target.line,
          endLine: target.endLine,
          column: target.column,
          key: ++jumpSequence,
        };
      } else if (!usedRawPath && target.heading) {
        tab.heading = { slug: target.heading, key: ++headingSequence };
      } else {
        tab.jump = { key: ++jumpSequence };
      }
    } catch (e) {
      if (ws.mounted) {
        ws.s.error = errorMessage(e);
        showRevealError();
      }
    }
  }
</script>

{#snippet readerHeader(
  tabs: CodeWorkspaceTab[],
  doc?: CodeWorkspaceTab,
  part: "all" | "tabs" | "commands" = "all",
)}
  <CodeWorkspaceHeader
    {part}
    bind:navOpen={ws.s.navOpen}
    dockNavigation={docked
      ? {
          show: () => dock?.show(docked!.files),
          shown: filesShown,
        }
      : undefined}
    navStatus={ws.s.navStatus}
    {tabs}
    context={ws.s.context}
    pending={ws.s.pending}
    {popout}
    activeMenu={doc?.menu}
    {section}
    activeQuick={doc?.quick}
    beginOpen={ws.beginOpen}
    supersedeOpenOnTabKey={ws.supersedeOpenOnTabKey}
    close={ws.close}
    refreshAll={() => {
      void ws.load();
      for (const tab of ws.s.tabs) void ws.refresh(tab);
    }}
    inspectablePath={doc?.rootKind === "artifacts"
      ? ""
      : (doc?.file?.document.path ?? doc?.diff?.new.path ?? "")}
    {onOpenGit}
    {onInspectGitFile}
  />
{/snippet}

{#snippet alert()}
  {#if notice || ws.s.error}<p
      role="alert"
      class="shrink-0 px-2 py-1 text-sm break-all text-destructive"
    >
      {notice || ws.s.error}
    </p>{/if}
{/snippet}

{#snippet documentView(tab: CodeWorkspaceTab, active: boolean)}
  <CodeDocumentView
    bind:this={
      () => views.get(tab.key),
      (view) => (view ? views.set(tab.key, view) : views.delete(tab.key))
    }
    restore={tab.restore}
    {id}
    {active}
    file={tab.file}
    diff={tab.diff}
    status={tab.status}
    paste={(text) => host.paste(text)}
    draftEnabled={host.draftEnabled}
    refresh={() => void ws.refresh(tab)}
    contextId={tab.contextId}
    comparison={tab.comparison}
    onComparison={(value) => (tab.comparison = value)}
    onState={(state) => (tab.status = state)}
    beginOpen={ws.beginOpen}
    openSide={(file, intent) => {
      ws.install(tab, file, undefined, intent === ws.selectionSequence);
    }}
    unified
    bind:quick={tab.quick}
    bind:menu={tab.menu}
    workspace={tab.contextId === ws.s.context?.contextId
      ? markdownWorkspace
      : undefined}
    heading={tab.heading}
    onHeading={() => (tab.heading = undefined)}
    jump={tab.jump}
    onJump={() => (tab.jump = undefined)}
    diffAvailable={tab.rootKind !== "artifacts"}
  />
{/snippet}

{#snippet navigation()}
  <CodeNavigator
    bind:this={navigator}
    restore={seed?.navigator}
    dropHost={docked ? navigationHost : section}
    bind:dropStatus
    {id}
    context={ws.s.context}
    listing={ws.s.listing}
    rootListing={ws.s.rootListing}
    fetchDirectory={ws.fetchDirectory}
    treeRefresh={ws.s.treeRefresh}
    {activePath}
    changes={ws.s.changes}
    changesError={ws.s.changesError}
    pending={ws.s.pending}
    sessionCwd={host.sessionCwd}
    reportedCwd={host.reportedCwd}
    navStatus={ws.s.navStatus}
    root={ws.s.root}
    rootError={ws.s.rootError}
    switchRoot={ws.switchRoot}
    retryRoot={() => void ws.loadContext()}
    reload={() => void ws.load()}
    changed={ws.filesChanged}
    changesEnabled={shown && !ws.s.changesError && !ws.s.navStatus}
    bind:nav={ws.s.nav}
    bind:query={ws.s.query}
    bind:directory={ws.s.directory}
    bind:includeIgnored={ws.s.includeIgnored}
    bind:changeGroup={ws.s.changeGroup}
    bind:changeQuery={ws.s.changeQuery}
    bind:changeSearch={ws.s.changeSearch}
    load={(append) => ws.load(append)}
    openFile={(path, opaqueId) => void openFile(path, opaqueId)}
    openRight={docked ? openRight : undefined}
    {locate}
    openDiff={(change) => void openDiff(change)}
  />
{/snippet}

{#if docked}
  {#each ws.s.tabs as doc (doc.key)}
    <CodeReaderGroup
      panel={readers.panel(doc.key)!}
      title={labels.get(doc.key) ?? doc.key}
      {doc}
      onclose={() => ws.close(doc.key)}
      onactivate={() => activateReader(doc.key)}
    >
      {#snippet header()}{@render readerHeader(
          [],
          doc,
          dock?.compact() ? "all" : "tabs",
        )}{/snippet}
      {#snippet commands()}{@render readerHeader([], doc, "commands")}{/snippet}
      <section
        {@attach (element) => {
          readingAreas.set(doc.key, element);
          return () => readingAreas.delete(doc.key);
        }}
        {inert}
        class="code-panel relative flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background"
        aria-label={m.s5_code_workspace()}
      >
        {#if !filesShown && alertTarget === doc}{@render alert()}{/if}
        <div class="min-h-0 flex-1 overflow-hidden">
          {@render documentView(doc, visibleDocument(doc))}
        </div>
      </section>
    </CodeReaderGroup>
  {/each}
  <CodeNavigationPanel id={docked.files} {inert} bind:navigationHost>
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      {#if filesShown}{@render alert()}{/if}
      {@render navigation()}
    </div>
  </CodeNavigationPanel>
{:else}
  <Tabs.Root value={ws.s.active} onValueChange={activateReader}>
    {#snippet child({ props })}
      <section
        {...props}
        bind:this={section}
        {inert}
        class="code-panel relative flex h-full min-h-0 min-w-0 flex-col overflow-hidden bg-background"
        aria-label={m.s5_code_workspace()}
      >
        {@render readerHeader(ws.s.tabs, activeTab)}
        {@render alert()}
        {#if dropStatus && !ws.s.navOpen}<p
            role="status"
            class="shrink-0 px-2 py-1 text-xs text-muted-foreground"
          >
            {dropStatus}
          </p>{/if}
        <CodeWorkspaceSplit
          bind:width={ws.s.navWidth}
          open={ws.s.navOpen}
          {navigation}
        >
          <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
            {#each ws.s.tabs as tab (tab.key)}
              <Tabs.Content value={tab.key} tabindex={-1}>
                {#snippet child({ props })}
                  <div {...props} class="min-h-0 flex-1 overflow-hidden">
                    {@render documentView(
                      tab,
                      shown && ws.s.active === tab.key,
                    )}
                  </div>
                {/snippet}
              </Tabs.Content>
            {:else}
              <p class="p-4 text-sm text-muted-foreground">
                {m.s5_workspace_empty_prompt()}
              </p>
            {/each}
          </div>
        </CodeWorkspaceSplit>
      </section>
    {/snippet}
  </Tabs.Root>
{/if}
