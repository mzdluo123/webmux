<script lang="ts">
  import GitBranchesPane from "../../src/components/workspace/GitBranchesPane.svelte";
  import GitOperationsPopover from "../../src/components/workspace/GitOperationsPopover.svelte";
  import { Button } from "../../src/lib/components/ui/button";
  import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";

  const fixture = buildGitWorkbenchFixture("branches-occupied");
  let backs = $state(0);
  tvMode.enabled = true;
  useTvNavigation(() => {
    backs++;
    return true;
  });
</script>

<div style="width:1100px;height:600px;display:flex;gap:16px">
  <div style="width:850px;height:600px">
    <GitBranchesPane
      context={fixture.context}
      branches={fixture.branches}
      onFilterInLog={() => {}}
      onRequestCheckout={() => {}}
      onOpenOccupiedWorktree={() => {}}
      onOpenPush={() => {}}
    />
  </div>
  <div>
    <Button>相邻工作区控件</Button>
    <GitOperationsPopover
      operations={fixture.operations}
      onRefreshOperations={() => {}}
    />
  </div>
</div>
<output aria-label="路由返回">{backs}</output>
