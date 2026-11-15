<script lang="ts">
  import { Label } from "$lib/components/ui/label";
  import { Slider } from "$lib/components/ui/slider";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as m from "$lib/paraglide/messages.js";

  let {
    open = $bindable(),
    fontSize,
    setFontSize,
    onCloseAutoFocus,
  }: {
    open: boolean;
    fontSize: number;
    setFontSize: (value: number) => void;
    onCloseAutoFocus: (event: Event) => void;
  } = $props();
</script>

<Dialog.Root bind:open>
  <Dialog.Content {onCloseAutoFocus}>
    <Dialog.Header
      ><Dialog.Title>{m.s4_font_size()}</Dialog.Title><Dialog.Description
        >{m.s4_font_size_desc()}</Dialog.Description
      ></Dialog.Header
    >
    <Label for="terminal-font">{fontSize}px</Label>
    <Slider
      id="terminal-font"
      type="single"
      min={8}
      max={20}
      step={1}
      value={fontSize}
      onValueChange={setFontSize}
      aria-label={m.s4_font_size()}
    />
  </Dialog.Content>
</Dialog.Root>
