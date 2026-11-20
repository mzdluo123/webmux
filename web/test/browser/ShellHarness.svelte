<script lang="ts">
  import { onMount, untrack } from "svelte";
  import AppShell from "../../src/components/shell/AppShell.svelte";
  import QuickStart from "../../src/pages/QuickStart.svelte";
  import WorkspaceSessionList from "../../src/pages/WorkspaceSessionList.svelte";
  import SessionList from "../../src/pages/SessionList.svelte";
  import { featureEnabled } from "../../src/platform/feature-host.svelte";
  import { matchRoute } from "../../src/hooks/shared/route";
  import type { TerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
  import type { Shell } from "../../src/hooks/shared/shell.svelte";
  import { provideSessionSwitcher } from "../../src/hooks/shared/session-switcher";
  import ShellSessionPage from "./ShellSessionPage.svelte";
  import Terminal from "../../src/pages/Terminal.svelte";

  // A shell page as App renders it, following the address: the home page,
  // quick start, or a stand-in for a session page inside the app shell.
  let {
    drafts,
    exposeShell,
    terminalPage = false,
    onSessionSwitcher,
  }: {
    drafts: TerminalDrafts;
    exposeShell?: (shell: Shell) => void;
    terminalPage?: boolean;
    onSessionSwitcher?: (invoker: HTMLElement | null) => void;
  } = $props();
  if (untrack(() => onSessionSwitcher))
    provideSessionSwitcher((invoker) => onSessionSwitcher?.(invoker));
  let path = $state(location.pathname);
  onMount(() => {
    const change = () => (path = location.pathname);
    window.addEventListener("popstate", change);
    return () => window.removeEventListener("popstate", change);
  });
  const route = $derived.by(() => {
    const matched = matchRoute(path);
    return matched.name === "terminal" || matched.name === "quick-start"
      ? matched
      : ({ name: "sessions" } as const);
  });
</script>

<AppShell {route} {drafts}>
  {#if route.name === "quick-start"}<QuickStart />
  {:else if route.name === "terminal" && terminalPage}<Terminal
      id={route.id}
      {drafts}
      onUploadNotice={() => {}}
    />
  {:else if route.name === "terminal"}<ShellSessionPage
      id={route.id}
      expose={exposeShell}
    />
  {:else if featureEnabled("workspace")}<WorkspaceSessionList />
  {:else}<SessionList />{/if}
</AppShell>
