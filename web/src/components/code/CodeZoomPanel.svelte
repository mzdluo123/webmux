<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import {
    codeFont,
    defaultCodeFontSize,
    maxCodeFontSize,
    minCodeFontSize,
  } from "../../hooks/code/code-font-preference.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Slider } from "$lib/components/ui/slider";
  import IconX from "~icons/tabler/x";

  let { onclose }: { onclose: () => void } = $props();
</script>

<div data-code-zoom class="flex items-center gap-2 px-2 py-1 text-xs">
  <span class="shrink-0 text-muted-foreground">{m.s5_font_size_label()}</span>
  <Slider
    type="single"
    min={minCodeFontSize}
    max={maxCodeFontSize}
    step={1}
    value={codeFont.size}
    onValueChange={(value) => codeFont.set(value)}
    aria-label={m.s5_code_font_size()}
    class="min-w-24 flex-1"
  />
  <output
    class="min-w-9 text-right tabular-nums"
    aria-label={m.s5_current_code_font_size()}>{codeFont.size}px</output
  >
  {#if codeFont.size !== defaultCodeFontSize}
    <Button variant="ghost" size="sm" onclick={() => codeFont.reset()}
      >{m.s5_reset()}</Button
    >
  {/if}
  <Button
    variant="ghost"
    size="icon-sm"
    aria-label={m.s5_close_zoom()}
    title={m.s5_close_zoom_esc()}
    onclick={onclose}><IconX aria-hidden="true" /></Button
  >
</div>
