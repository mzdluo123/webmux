<script lang="ts">
  import * as RadioGroup from "$lib/components/ui/radio-group";
  import * as m from "$lib/paraglide/messages.js";
  import type { ChatSupport, SessionView } from "../../contracts/schemas";
  import { connectorNoticeText } from "../../domains/sessions/hybrid";

  let {
    startSide = $bindable(),
    support,
    disabled = false,
  }: {
    startSide: SessionView;
    support: ChatSupport;
    disabled?: boolean;
  } = $props();
</script>

<RadioGroup.Root
  bind:value={
    () => startSide,
    (next) => {
      if (next === "terminal" || next === "chat") startSide = next;
    }
  }
  aria-label={m.s3_start_side_label()}
  {disabled}
>
  <RadioGroup.Item
    value="terminal"
    aria-label={m.s3_start_from_terminal()}
    title={m.s3_start_from_terminal()}
    description={m.s3_start_from_terminal_desc()}
  />
  <RadioGroup.Item
    value="chat"
    aria-label={m.s3_start_from_chat()}
    title={m.s3_start_from_chat()}
    description={m.s3_start_from_chat_desc()}
  />
</RadioGroup.Root>
{#if support.connector}
  <p class="text-xs text-muted-foreground">
    {m.s3_sentence_end({ text: connectorNoticeText(support.connector) })}
  </p>
{/if}
