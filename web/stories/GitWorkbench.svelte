<script lang="ts">
  import GitWorkbenchShell from "../src/components/workspace/GitWorkbenchShell.svelte";
  import {
    buildGitWorkbenchFixture,
    type GitWorkbenchScenario,
  } from "./git-workbench-fixtures";

  let {
    scenario = "log-merge",
    stressLongText = false,
  }: {
    scenario?: GitWorkbenchScenario;
    stressLongText?: boolean;
  } = $props();

  let view = $derived(buildGitWorkbenchFixture(scenario, stressLongText));
</script>

<main
  class="mx-auto flex min-h-[min(52rem,92dvh)] w-full max-w-7xl flex-col p-2 sm:p-4 xl:h-[min(52rem,92dvh)]"
  data-testid="git-workbench-story"
>
  <GitWorkbenchShell bind:view />
</main>
