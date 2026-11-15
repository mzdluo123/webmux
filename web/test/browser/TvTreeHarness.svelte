<script lang="ts">
  import { onDestroy } from "svelte";
  import WorkbenchTree from "../../src/components/shell/WorkbenchTree.svelte";
  import { Button } from "../../src/lib/components/ui/button";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
  import { repository, session } from "./shell-fixture";
  tvMode.enabled = true;
  useTvNavigation(() => false);
  onDestroy(() => {
    tvMode.enabled = false;
  });
  const repositories = [repository()];
  const sessions = [
    session({
      title: "TV leaf",
      workspace: {
        repositoryId: "repo1",
        worktreeId: "main",
        worktreePath: "/repos/webmux",
      },
    }),
  ];
</script>

<div style="position:fixed;left:0;top:0;width:280px">
  <WorkbenchTree
    {repositories}
    {sessions}
    records={[]}
    home={{ repo1: ["main"] }}
    scope=""
    current=""
    canCreate
    configIcons={new Map()}
    onscope={() => {}}
    onopen={() => {}}
    onnewworktree={() => {}}
    onmanageworktrees={() => {}}
    onremoveworktree={() => {}}
    onworktreedetails={() => {}}
    onswitchbranch={() => {}}
    onrename={() => {}}
    onclose={() => {}}
    onresume={() => {}}
    onremove={() => {}}
  />
</div>
<div style="position:fixed;left:500px;top:20px">
  <Button>主内容操作</Button>
</div>
