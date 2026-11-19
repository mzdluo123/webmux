<script lang="ts">
  import type { Snippet } from "svelte";
  import * as Tabs from "$lib/components/ui/tabs";
  import DockPanel from "../shared/DockPanel.svelte";
  import {
    codeReaderTitle,
    type CodeWorkspaceTab,
  } from "../../domains/code/code-workspace";

  let {
    panel,
    doc,
    activate,
    onactivate,
    header,
    children,
  }: {
    panel?: string;
    doc?: CodeWorkspaceTab;
    activate: (key: string) => void;
    onactivate: () => void;
    header: Snippet;
    children: Snippet<[Record<string, unknown>]>;
  } = $props();
</script>

<Tabs.Root value={doc?.key ?? ""} onValueChange={activate}>
  {#snippet child({ props })}
    {#if panel}
      <DockPanel
        id={panel}
        hostProps={props}
        title={codeReaderTitle(
          doc?.file?.document.path ?? doc?.diff?.new.path ?? "",
        )}
        {onactivate}
        {header}
      >
        {@render children({})}
      </DockPanel>
    {:else}
      {@render children(props)}
    {/if}
  {/snippet}
</Tabs.Root>
