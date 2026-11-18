<script lang="ts">
  import type { Snippet } from "svelte";

  let { target, children }: { target?: HTMLElement; children: Snippet } =
    $props();

  // Compact Git steps can combine two wide panels without remounting their contents.
  function place(node: HTMLDivElement) {
    const home = node.parentElement!;
    $effect(() => {
      if (!target) return;
      target.append(node);
      return () => home.append(node);
    });
  }
</script>

<div class="contents" use:place>
  {@render children()}
</div>
