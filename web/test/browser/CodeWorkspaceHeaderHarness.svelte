<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as Tabs from "$lib/components/ui/tabs";
  import CodeWorkspaceHeader from "../../src/components/code/CodeWorkspaceHeader.svelte";
  import type {
    CodeContext,
    CodeWorkspaceTab,
  } from "../../src/domains/code/code-workspace";
  import IconSearch from "~icons/tabler/search";

  let {
    context,
    inspectablePath,
    onFind,
    onInspectGitFile,
    onOpenGit,
    onRefresh = () => {},
    tabs = [],
    width = "720px",
  }: {
    context?: CodeContext;
    inspectablePath?: string;
    onFind?: () => void;
    onOpenGit?: () => void;
    onInspectGitFile?: (
      path: string,
      tab: "history" | "blame" | "compare",
    ) => void;
    onRefresh?: () => void;
    tabs?: CodeWorkspaceTab[];
    width?: string;
  } = $props();

  let navOpen = $state(true);
</script>

{#snippet quick()}
  <Button
    variant="ghost"
    size="icon-sm"
    aria-label="文件内查找"
    title="文件内查找"
    onclick={onFind}><IconSearch aria-hidden="true" /></Button
  >
{/snippet}

{#snippet menu()}
  <DropdownMenu.Item onSelect={onFind}>文件内查找</DropdownMenu.Item>
{/snippet}

<Tabs.Root value={tabs[0]?.key ?? ""}>
  <div class="flex" style:width>
    <CodeWorkspaceHeader
      bind:navOpen
      navStatus=""
      {context}
      {tabs}
      pending={false}
      activeQuick={onFind ? quick : undefined}
      activeMenu={onFind ? menu : undefined}
      beginOpen={() => {}}
      supersedeOpenOnTabKey={() => {}}
      close={() => {}}
      refreshAll={onRefresh}
      {inspectablePath}
      {onOpenGit}
      {onInspectGitFile}
    />
  </div>
</Tabs.Root>
