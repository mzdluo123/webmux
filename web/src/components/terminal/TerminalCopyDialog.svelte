<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Dialog from "$lib/components/ui/dialog";
  import { m } from "$lib/paraglide/messages.js";

  // Manual copy when the clipboard cannot be written directly.
  let {
    open = $bindable(),
    text,
    onCloseAutoFocus,
  }: {
    open: boolean;
    text: string;
    onCloseAutoFocus: (event: Event) => void;
  } = $props();
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="sm:max-w-lg" {onCloseAutoFocus}>
    <Dialog.Header
      ><Dialog.Title>{m.s4_copy_dialog_title()}</Dialog.Title
      ><Dialog.Description>{m.s4_copy_dialog_desc()}</Dialog.Description
      ></Dialog.Header
    >
    <Textarea
      aria-label={m.s4_copy_content_aria()}
      readonly
      value={text}
      mono
      class="max-h-[50dvh] min-h-40 overflow-y-auto"
    />
    <Dialog.Footer
      ><Button
        onclick={() => {
          open = false;
        }}>{m.s4_close()}</Button
      ></Dialog.Footer
    >
  </Dialog.Content>
</Dialog.Root>
