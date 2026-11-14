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
    reveal = () => {},
    observe = async () => {},
    unobserve = () => {},
    ready = () => true,
  }: {
    source: XrSessionSource;
    errors: string[];
    expose: (header: XrHeaderControl) => void;
    reveal?: (id: string) => void | Promise<void>;
    observe?: (id: string) => Promise<void>;
    unobserve?: (id: string) => void;
    ready?: () => boolean;
  } = $props();
  let activeSession = $state("other-session");
  const workspace = createXrSessionWorkspace({
    available: () => [{ id: source.id, title: source.title }],
    observe: (id) => observe(id),
    unobserve: (id) => unobserve(id),
    ready: () => ready(),
    reveal: async (id) => {
      await reveal(id);
      activeSession = id;
    },
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
<section aria-label="Active session">{activeSession}</section>
<button onclick={header.enter}>进入 VR</button>
<button onclick={header.exit}>退出 VR</button>
<button onclick={() => void workspace.returnTo2D(source.id)}
  >返回 2D 原会话</button
>
