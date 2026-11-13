<script lang="ts">
  import * as Tabs from "$lib/components/ui/tabs";
  import CodeWorkspaceHeader from "../../src/components/code/CodeWorkspaceHeader.svelte";

  let {
    onOpenGit,
    onRefresh = () => {},
  }: {
    onOpenGit?: () => void;
    onRefresh?: () => void;
  } = $props();

  let navOpen = $state(true);
</script>

<Tabs.Root value="">
  <CodeWorkspaceHeader
    bind:navOpen
    navStatus=""
    tabs={[]}
    pending={false}
    error=""
    dropStatus=""
    beginOpen={() => {}}
    supersedeOpenOnTabKey={() => {}}
    close={() => {}}
    refreshAll={onRefresh}
    {onOpenGit}
  />
</Tabs.Root>
