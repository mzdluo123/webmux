<script lang="ts">
  import IconNotes from "~icons/tabler/notes";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import { cn } from "$lib/utils.js";
  import type { TerminalDraft } from "../../domains/terminal/terminal-draft.svelte";

  // The draft-mode switch. Wide screens keep it in the header; phones keep it
  // beside the soft keys, where input actions live. An icon like its
  // neighbours; green while draft mode is on, a dot when scratch notes wait.
  let {
    draft,
    draftSaved,
    scratchSaved,
    disabled,
    toggle,
    class: className,
  }: {
    draft: TerminalDraft;
    draftSaved: string;
    scratchSaved: string;
    disabled: boolean;
    toggle: () => void;
    class?: string;
  } = $props();
</script>

<Button
  variant={draft.enabled ? "success" : "ghost"}
  size="icon-sm"
  class={cn("relative", className)}
  aria-label={draft.scratch.length
    ? m.s4_draft_mode_with_scratch({ count: String(draft.scratch.length) })
    : m.s4_draft_mode()}
  aria-pressed={draft.enabled}
  {disabled}
  title={draft.enabled
    ? m.s4_draft_hide_only_hint()
    : draft.scratchUpdatedWhileHidden
      ? m.s4_draft_scratch_updated_hint({ saved: scratchSaved })
      : draft.scratch.length
        ? m.s4_draft_view_with_scratch_hint({
            count: String(draft.scratch.length),
            saved: scratchSaved,
          })
        : draft.updatedWhileHidden
          ? m.s4_draft_hidden_updated_hint({ saved: draftSaved })
          : draft.text
            ? m.s4_draft_view_kept_hint({ saved: draftSaved })
            : m.s4_enable_draft_mode()}
  onclick={toggle}
  ><IconNotes
    class="size-4"
    aria-hidden="true"
  />{#if !draft.enabled && draft.scratch.length}<span
      class="absolute top-0.5 right-0.5 size-1.5 rounded-full bg-warning"
      aria-hidden="true"
    ></span>{/if}</Button
>
