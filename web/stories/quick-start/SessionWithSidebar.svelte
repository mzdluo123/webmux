<script lang="ts">
  import HybridSession from "../acp/HybridSession.svelte";
  import AppShell from "./AppShell.svelte";
  import type { Mode } from "./data";

  // Prototype: the existing hybrid session page inside the app shell. The
  // sidebar follows the mode; switching mode here keeps the session open.
  // The narrow-screen sidebar button replaces the header's back button and
  // is not prototyped here.
  let { mode = "chat" }: { mode?: Mode } = $props();
</script>

<AppShell {mode} session="s1">
  {#snippet children()}
    <HybridSession scenario="conversation" agent="claudecode" />
  {/snippet}
</AppShell>
