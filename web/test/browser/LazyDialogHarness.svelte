<script lang="ts">
  import LazyDialog from "../../src/components/shared/LazyDialog.svelte";

  let {
    load,
    label = "端口转发",
    onclose,
  }: {
    load: () => Promise<string>;
    label?: string;
    onclose: () => void;
  } = $props();
</script>

<LazyDialog {load} {label} {onclose}>
  {#snippet children(content)}
    <div role="dialog" aria-label={label}>{content}</div>
  {/snippet}
</LazyDialog>
