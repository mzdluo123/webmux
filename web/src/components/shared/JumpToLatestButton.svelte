<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";

  let {
    class: className = "",
    labelClass = "",
    onclick,
  }: {
    class?: string;
    labelClass?: string;
    onclick: () => void;
  } = $props();
</script>

<Button
  variant="outline"
  size="sm"
  raised
  class={className}
  aria-label={m.s4_jump_to_latest()}
  title={m.s4_jump_to_latest()}
  onmousedown={(event) => event.preventDefault()}
  {onclick}
  ><IconArrowDown aria-hidden="true" /><span class={labelClass}
    >{m.s4_jump_to_latest()}</span
  ></Button
>
