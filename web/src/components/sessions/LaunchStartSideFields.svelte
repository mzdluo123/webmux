<script lang="ts">
  import * as RadioGroup from "$lib/components/ui/radio-group";
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
  aria-label="起始端"
  {disabled}
>
  <RadioGroup.Item
    value="terminal"
    aria-label="从终端开始"
    title="从终端开始"
    description="先在终端里运行，随时可切到聊天"
  />
  <RadioGroup.Item
    value="chat"
    aria-label="从聊天开始"
    title="从聊天开始"
    description="直接打开聊天界面，随时可切到终端"
  />
</RadioGroup.Root>
{#if support.connector}
  <p class="text-xs text-muted-foreground">
    {connectorNoticeText(support.connector)}。
  </p>
{/if}
