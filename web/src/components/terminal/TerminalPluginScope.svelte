<script lang="ts">
  import { untrack, type Snippet } from "svelte";
  import type { TerminalPlugins } from "../../plugins/terminal";
  import type { SessionInfo } from "../../contracts/schemas";

  let {
    plugins: initialPlugins,
    available,
    open,
    error,
    children,
  }: {
    plugins: TerminalPlugins;
    available: () => readonly SessionInfo[];
    open: (id: string) => Promise<void>;
    error: (message: string) => void;
    children: Snippet;
  } = $props();
  const plugins = untrack(() => initialPlugins);
  plugins.immersive?.createXrSessionWorkspace({
    available: () => available(),
    open: (id) => open(id),
    error: (message) => error(message),
  });
</script>

{@render children()}
