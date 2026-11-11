<script lang="ts">
  import type { GitIssue } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    blockers = [],
    warnings = [],
  }: {
    blockers?: readonly GitIssue[];
    warnings?: readonly GitIssue[];
  } = $props();
</script>

{#each blockers as blocker (blocker.code)}
  <InlineNotice tone="danger">{blocker.message}</InlineNotice>
{/each}

{#each warnings as warning (warning.code)}
  <InlineNotice tone="warning">{warning.message}</InlineNotice>
{/each}
