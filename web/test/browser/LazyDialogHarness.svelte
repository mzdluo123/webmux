<script lang="ts">
  import LazyDialog from "../../src/components/shared/LazyDialog.svelte";

  let {
    load,
    label = "端口转发",
    showLoading,
    onclose,
  }: {
    load: () => Promise<string>;
    label?: string;
    showLoading?: boolean;
    onclose: () => void;
  } = $props();
</script>

<LazyDialog {load} {label} {showLoading} {onclose}>
  {#snippet children(content)}
    <div role="dialog" aria-label={label}>{content}</div>
  {/snippet}
</LazyDialog>
