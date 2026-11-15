<script lang="ts">
  import { untrack, type Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { TerminalPlugins } from "../../plugins/terminal";
  import type { SessionInfo } from "../../contracts/schemas";

  let {
    plugins: initialPlugins,
    available,
    ready,
    panes,
    info,
    open,
    exited,
    error,
    observed = $bindable<string[]>([]),
    immersive = $bindable(false),
    children,
  }: {
    plugins: TerminalPlugins;
    available: () => readonly SessionInfo[];
    ready: () => boolean;
    panes: () => readonly string[];
    info: (id: string) => SessionInfo | undefined;
    open: (id: string) => Promise<void>;
    exited: () => void;
    error: (message: string) => void;
    observed?: string[];
    immersive?: boolean;
    children: Snippet;
  } = $props();
  const plugins = untrack(() => initialPlugins);
  const membership = plugins.immersive?.createXrSessionMembership({
    live: () => available(),
    ready: () => ready(),
    panes: () => panes(),
    info: (id) => info(id),
  });
  const xr = plugins.immersive?.createXrSessionWorkspace({
    available: () => membership!.available,
    ready: () => ready(),
    observe: async (id) => {
      if (!xr?.active) throw new Error(m.s4_vr_workspace_exited());
      await membership!.observe(id);
    },
    unobserve: (id) => membership!.unobserve(id),
    reveal: (id) => open(id),
    exited: () => {
      membership!.clear();
      exited();
    },
    error: (message) => error(message),
  });
  $effect(() => {
    observed = membership?.observed ?? [];
    immersive = xr?.active ?? false;
  });
  export function unobserve(id: string) {
    membership?.unobserve(id);
  }
</script>

{@render children()}
