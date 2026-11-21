<script lang="ts">
  import LazyPage from "../../src/components/shared/LazyPage.svelte";

  let {
    load,
    label = "设置",
    loadingDelayMs,
  }: {
    load: () => Promise<string>;
    label?: string;
    loadingDelayMs?: number;
  } = $props();
</script>

<LazyPage {load} {label} {loadingDelayMs}>
  {#snippet children(text)}
    <main>{text}</main>
  {/snippet}
</LazyPage>
