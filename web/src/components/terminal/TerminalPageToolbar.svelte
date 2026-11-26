<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type DockWorkspace from "../shared/DockWorkspace.svelte";
  import SessionWorkspaceHeader from "./SessionWorkspaceHeader.svelte";
  import TerminalPageNotices from "./TerminalPageNotices.svelte";

  let {
    dock,
    closePane,
    pickerOpen,
    openPanels,
    fileLabel,
    openFiles,
    prefix,
    featureErrors,
    error,
    onerror,
    dismissError,
  }: {
    dock?: DockWorkspace;
    closePane: (panel: string) => void;
    pickerOpen: boolean;
    openPanels: () => void;
    fileLabel: string;
    openFiles?: () => void;
    prefix: boolean;
    featureErrors?: string[];
    error: string;
    onerror: (message: string) => void;
    dismissError: () => void;
  } = $props();
</script>

<div class="session-toolbar shrink-0">
  <SessionWorkspaceHeader
    {dock}
    {closePane}
    {pickerOpen}
    {openPanels}
    {fileLabel}
    {openFiles}
    {onerror}
  />
</div>
{#if prefix}
  <p
    class="shrink-0 border-b px-2 py-1 text-xs text-muted-foreground"
    role="status"
  >
    {m.s4_prefix_shortcut_bar()}
  </p>
{/if}
<TerminalPageNotices
  {featureErrors}
  {error}
  reloadPage={() => location.reload()}
  {dismissError}
/>
