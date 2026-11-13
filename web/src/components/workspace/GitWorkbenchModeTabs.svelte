<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconGitCommit from "~icons/tabler/git-commit";
  import IconHistory from "~icons/tabler/history";
  import { Badge } from "$lib/components/ui/badge";
  import * as Tabs from "$lib/components/ui/tabs";

  let { stashCount = 0 }: { stashCount?: number } = $props();
</script>

<div class="flex min-w-0 shrink-0 items-center border-b px-2">
  <Tabs.List
    variant="line"
    animated
    class="h-auto min-h-8 max-w-full flex-wrap group-data-horizontal/tabs:h-auto"
    aria-label="Git 工作台视图"
  >
    <Tabs.Trigger value="log">
      <IconHistory aria-hidden="true" />
      <span>历史</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="commit">
      <IconGitCommit aria-hidden="true" />
      <span>提交</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="branches">
      <IconGitBranch aria-hidden="true" />
      <span>分支</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="stash" data-testid="git-mode-tab-stash">
      <IconArchive aria-hidden="true" />
      <span>贮藏</span>
      {#if stashCount > 0}
        <Badge variant="outline">{stashCount}</Badge>
      {/if}
    </Tabs.Trigger>
  </Tabs.List>
</div>
