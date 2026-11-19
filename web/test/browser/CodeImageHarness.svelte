<script lang="ts">
  import type { Snippet } from "svelte";
  import * as DropdownMenu from "../../src/lib/components/ui/dropdown-menu";
  import CodeDocumentView from "../../src/components/code/CodeDocumentView.svelte";
  import type { CodeDiff, CodeFile } from "../../src/contracts/schemas";
  let {
    file,
    diff,
    active = true,
  }: { file?: CodeFile; diff?: CodeDiff; active?: boolean } = $props();
  let status = $state("unchanged");
  let inserted = $state("");
  let menu = $state<Snippet>();
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>代码菜单</DropdownMenu.Trigger>
  <DropdownMenu.Content>{@render menu?.()}</DropdownMenu.Content>
</DropdownMenu.Root>
<div data-testid="frame" style="height:650px;width:650px">
  <CodeDocumentView
    id="session"
    {active}
    {file}
    {diff}
    {status}
    contextId="context"
    draftEnabled={false}
    paste={(text) => {
      inserted = text;
      return "terminal";
    }}
    refresh={() => {}}
    beginOpen={() => 1}
    openSide={() => {}}
    onComparison={() => {}}
    onState={(value) => (status = value)}
    bind:menu
  />
</div>
<p aria-label="Inserted reference">{inserted}</p>
