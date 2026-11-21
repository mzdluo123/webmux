<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import CodeWorkspace from "../components/code/CodeWorkspace.svelte";
  import { useCodePopoutFrame } from "../hooks/code/code-popout-frame.svelte";

  let { id }: { id: string } = $props();
  const frame = useCodePopoutFrame();
  let workspace = $state<CodeWorkspace>();
  $effect(() => {
    const current = workspace;
    if (current) frame.ready(() => current.capture());
  });
</script>

<main class="flex h-dvh min-h-0 flex-col">
  {#if frame.phase === "ready"}
    <CodeWorkspace
      bind:this={workspace}
      {id}
      visible
      host={frame.host}
      initial={frame.initial}
      popout={{ popped: true, toggle: () => frame.close() }}
    />
  {:else if frame.phase === "standalone"}
    <div class="space-y-2 p-6 text-sm">
      <p>{m.s1_code_popout_requires_session()}</p>
      <Button href={`/s/${id}`} variant="outline" size="sm"
        >{m.s1_back_to_session()}</Button
      >
    </div>
  {/if}
</main>
