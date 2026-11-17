<script lang="ts">
  import { onMount } from "svelte";
  import Harness from "./ChatViewHarness.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  let { id }: { id: string } = $props();
  tvMode.enabled = true;
  useTvNavigation(() => false);
  onMount(() => () => {
    tvMode.enabled = false;
  });
</script>

<main><Harness {id} /></main>
