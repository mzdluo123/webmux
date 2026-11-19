<script lang="ts">
  import IconPin from "~icons/tabler/pin";
  import { m } from "$lib/paraglide/messages.js";

  let {
    srLabel = false,
    size = "sm",
  }: {
    /** Announces the pinned state where the surrounding control does not already name it. */
    srLabel?: boolean;
    size?: "sm" | "md";
  } = $props();
</script>

<span
  class="inline-flex shrink-0 items-center text-muted-foreground"
  data-pinned-worktree
  title={m.s8_badge_pinned_worktree()}
  ><IconPin
    class={size === "md" ? "size-4" : "size-3.5"}
    aria-hidden="true"
  />{#if srLabel}<span class="sr-only">{m.s8_badge_pinned_worktree()}</span
    >{/if}</span
>
