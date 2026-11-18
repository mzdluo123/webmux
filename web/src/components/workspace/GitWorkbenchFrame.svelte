<script lang="ts">
  import type { Snippet } from "svelte";
  import { ScrollArea } from "$lib/components/ui/scroll-area";

  let { children }: { children: Snippet } = $props();
</script>

<!-- On short screens the chrome can scroll instead of squeezing the dock. -->
<ScrollArea
  class="flex-1"
  viewportClasses="[&>[data-scroll-area-content]]:h-full"
>
  <div class="flex h-full min-h-0 min-w-0 flex-col">
    {@render children()}
  </div>
</ScrollArea>
