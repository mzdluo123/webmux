<script lang="ts">
  import HybridSession from "../acp/HybridSession.svelte";
  import ChatShell from "./ChatShell.svelte";

  // Prototype: the existing hybrid session page beside the chat sidebar, the
  // current chat marked. The mobile drawer button belongs in the session
  // header and is not prototyped here.
</script>

<ChatShell current="s1">
  {#snippet children()}
    <HybridSession scenario="conversation" agent="claudecode" />
  {/snippet}
</ChatShell>
