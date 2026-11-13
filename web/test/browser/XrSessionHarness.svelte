<script lang="ts">
  import { onMount, untrack } from "svelte";
  import type {
    XrHeaderControl,
    XrSessionSource,
  } from "../../src/domains/terminal/xr-workspace";
  import { createXrSessionWorkspace } from "../../src/hooks/terminal/xr-session.svelte";

  let {
    source,
    errors,
    expose,
  }: {
    source: XrSessionSource;
    errors: string[];
    expose: (header: XrHeaderControl) => void;
  } = $props();
  const workspace = createXrSessionWorkspace({
    available: () => [{ id: source.id, title: source.title }],
    open: () => {},
    error: (message) => errors.push(message),
  });
  const header = workspace.header(untrack(() => source.id));
  untrack(() => expose(header));
  onMount(() => workspace.register(source));
</script>

<p role="status" aria-label="XR state">
  {header.supported ? "supported" : "unsupported"}
  {header.active ? "immersive" : "2D"}
  {header.busy ? "busy" : "idle"}
</p>
<button onclick={header.enter}>进入 VR</button>
<button onclick={header.exit}>退出 VR</button>
