<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as m from "$lib/paraglide/messages.js";

  // Manual paste when the clipboard cannot be read directly.
  let {
    open = $bindable(),
    text = $bindable(),
    filePaste,
    send,
    onCloseAutoFocus,
  }: {
    open: boolean;
    text: string;
    filePaste: (event: ClipboardEvent) => boolean;
    send: (text: string) => void;
    onCloseAutoFocus: (event: Event) => void;
  } = $props();
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="sm:max-w-lg" {onCloseAutoFocus}>
    <Dialog.Header
      ><Dialog.Title>{m.s4_paste_dialog_title()}</Dialog.Title
      ><Dialog.Description>{m.s4_paste_dialog_desc()}</Dialog.Description
      ></Dialog.Header
    >
    <Textarea
      aria-label={m.s4_paste_content_aria()}
      onpaste={(event) => {
        if (filePaste(event)) open = false;
      }}
      bind:value={text}
      code
      class="max-h-[50dvh] min-h-40 overflow-y-auto"
    />
    <Dialog.Footer
      ><Button
        variant="outline"
        onclick={() => {
          open = false;
        }}>{m.s4_cancel()}</Button
      ><Button
        onclick={() => {
          send(text);
          open = false;
        }}>{m.s4_send()}</Button
      ></Dialog.Footer
    >
  </Dialog.Content>
</Dialog.Root>
