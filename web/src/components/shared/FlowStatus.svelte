<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import type { PreviewFlow } from "../../hooks/workspace/preview-flow.svelte";
  import InlineNotice from "./InlineNotice.svelte";

  // What a branch flow's preview is doing: reading, failed to read, or read
  // again after an execution failed or found the state changed.
  let {
    flow,
  }: {
    flow: PreviewFlow<unknown>;
  } = $props();
</script>

{#if flow.stale}
  <InlineNotice tone="warning"
    >{m.s1_flow_stale_repreviewed({ stale: flow.stale })}</InlineNotice
  >
{:else if flow.failure}
  <InlineNotice tone="danger"
    >{m.s1_flow_failed_repreviewed({ failure: flow.failure })}</InlineNotice
  >
{/if}
{#if flow.error}
  <InlineNotice tone="danger"
    >{m.s1_flow_preview_error({ error: flow.error })}</InlineNotice
  >
  <Button
    size="sm"
    variant="outline"
    disabled={flow.loading}
    onclick={() => void flow.refresh()}>{m.s1_flow_refresh_preview()}</Button
  >
{:else if flow.loading && !flow.preview}
  <p role="status" class="text-xs text-muted-foreground">
    {m.s1_flow_loading_preview()}
  </p>
{/if}
