<script lang="ts">
  import IconCircle from "~icons/tabler/circle";
  import IconCircleCheck from "~icons/tabler/circle-check";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { SwitchStep, SwitchTarget } from "../../domains/sessions/hybrid";

  // A switch between the chat and the terminal: the side being left must
  // exit before the other starts, and input stays disabled throughout.
  let {
    target,
    steps,
    awaitingChoice,
    onchoose,
  }: {
    target: SwitchTarget | null;
    steps: SwitchStep[];
    /** The interrupt timed out: the user picks how to go on (D21). */
    awaitingChoice: boolean;
    /** Reopens the choice after its dialog was closed. */
    onchoose: () => void;
  } = $props();
</script>

<div
  class="flex h-full flex-col items-center justify-center gap-4 overflow-y-auto bg-background p-6 motion-safe:animate-in motion-safe:fade-in-0 motion-safe:duration-200"
  role="status"
  aria-live="polite"
>
  <IconLoader2
    class="size-8 shrink-0 animate-spin text-muted-foreground"
    aria-hidden="true"
  />
  <h2 class="text-center text-base font-medium">
    {target === "terminal"
      ? m.s3_switching_to_terminal()
      : target === "chat"
        ? m.s3_switching_to_chat()
        : m.s3_switching_generic()}
  </h2>
  <ol class="w-full max-w-xs space-y-2 text-sm">
    {#each steps as step (step.label)}
      <li
        class={step.state === "pending"
          ? "flex items-center gap-2 text-muted-foreground"
          : "flex items-center gap-2"}
      >
        {#if step.state === "done"}
          <IconCircleCheck
            class="size-4 shrink-0 text-success"
            aria-label={m.s3_status_completed()}
          />
        {:else if step.state === "current"}
          <IconLoader2
            class="size-4 shrink-0 animate-spin text-info"
            aria-label={m.s3_status_in_progress()}
          />
        {:else}
          <IconCircle
            class="size-4 shrink-0"
            aria-label={m.s3_row_status_waiting()}
          />
        {/if}
        <span class="min-w-0 break-words">{step.label}</span>
      </li>
    {/each}
  </ol>
  {#if awaitingChoice}
    <p class="max-w-xs text-center text-sm text-warning">
      {m.s3_switch_tui_not_stopped()}
    </p>
    <Button variant="outline" size="sm" onclick={onchoose}
      >{m.s3_choose_ellipsis()}</Button
    >
  {/if}
  <p class="max-w-xs text-center text-xs text-muted-foreground">
    {m.s3_switch_input_paused_hint()}
  </p>
</div>
