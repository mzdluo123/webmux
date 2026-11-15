<script lang="ts">
  import IconArrowDown from "~icons/tabler/arrow-down";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";

  let {
    class: className = "",
    onclick,
  }: {
    class?: string;
    onclick: () => void;
  } = $props();
</script>

<Button
  variant="outline"
  size="sm"
  raised
  class={className}
  onmousedown={(event) => event.preventDefault()}
  {onclick}><IconArrowDown aria-hidden="true" />{m.s4_jump_to_latest()}</Button
>
