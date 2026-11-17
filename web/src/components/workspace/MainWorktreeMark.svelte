<script lang="ts">
  import IconHome from "~icons/tabler/home";
  import { m } from "$lib/paraglide/messages.js";

  let {
    srLabel = false,
    size = "sm",
  }: {
    /** Announces the main worktree where the surrounding control does not already name it. */
    srLabel?: boolean;
    size?: "sm" | "md";
  } = $props();
</script>

<span
  class="inline-flex shrink-0 items-center text-muted-foreground"
  data-main-worktree
  title={m.s8_badge_main_worktree()}
  ><IconHome
    class={size === "md" ? "size-4" : "size-3.5"}
    aria-hidden="true"
  />{#if srLabel}<span class="sr-only">{m.s8_badge_main_worktree()}</span
    >{/if}</span
>
