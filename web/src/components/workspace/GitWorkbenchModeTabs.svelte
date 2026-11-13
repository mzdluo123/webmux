<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconGitCommit from "~icons/tabler/git-commit";
  import IconHistory from "~icons/tabler/history";
  import { Badge } from "$lib/components/ui/badge";
  import * as Tabs from "$lib/components/ui/tabs";

  let {
    desktop = false,
    stashCount = 0,
  }: {
    desktop?: boolean;
    stashCount?: number;
  } = $props();
</script>

<div
  class={desktop
    ? "flex shrink-0 flex-col border-r bg-background p-1.5"
    : "flex min-w-0 shrink-0 items-center border-b bg-background px-2"}
>
  <Tabs.List
    variant="line"
    class="h-auto min-h-8 max-w-full flex-wrap group-data-horizontal/tabs:h-auto"
    aria-label="Git 工作台视图"
  >
    <Tabs.Trigger value="log">
      <IconHistory aria-hidden="true" />
      <span>历史</span>
      <span class="max-sm:hidden">(Log)</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="commit">
      <IconGitCommit aria-hidden="true" />
      <span>提交</span>
      <span class="max-sm:hidden">(Commit)</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="branches">
      <IconGitBranch aria-hidden="true" />
      <span>分支</span>
      <span class="max-sm:hidden">(Branches)</span>
    </Tabs.Trigger>
    <Tabs.Trigger value="stash" data-testid="git-mode-tab-stash">
      <IconArchive aria-hidden="true" />
      <span>贮藏</span>
      <span class="max-sm:hidden">(Stash)</span>
      {#if stashCount > 0}
        <Badge variant="outline">{stashCount}</Badge>
      {/if}
    </Tabs.Trigger>
  </Tabs.List>
</div>
