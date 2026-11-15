<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import IconX from "~icons/tabler/x";

  let {
    line,
    onsubmit,
    onclose,
  }: {
    line: number;
    onsubmit: (value: string) => void;
    onclose: () => void;
  } = $props();
  // svelte-ignore state_referenced_locally
  let value = $state(String(line));
</script>

<form
  data-code-goto-line
  class="flex items-center gap-1 p-1"
  onsubmit={(e) => {
    e.preventDefault();
    onsubmit(value);
  }}
>
  <Input
    bind:value
    aria-label={m.s5_goto_line_aria()}
    placeholder={m.s5_goto_line_placeholder()}
    inputmode="numeric"
    class="h-7 flex-1"
    onkeydown={(e) => {
      if (e.key !== "Escape") return;
      e.preventDefault();
      onclose();
    }}
  />
  <Button type="submit" size="sm">{m.s5_go()}</Button>
  <Button
    type="button"
    variant="ghost"
    size="icon-sm"
    aria-label={m.s5_close_goto_line()}
    title={m.s5_close_goto_line_esc()}
    onclick={onclose}><IconX aria-hidden="true" /></Button
  >
</form>
