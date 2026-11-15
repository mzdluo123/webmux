<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import {
    namingDialogDescription,
    type FileTarget,
  } from "../../domains/code/code-file-actions";

  let {
    kind,
    node,
    name = $bindable(""),
    nameInput = $bindable(null),
    busy,
    dialogError,
    onsubmit,
    onclose,
  }: {
    kind: "rename" | "mkdir";
    node: FileTarget;
    name: string;
    nameInput: HTMLInputElement | null;
    busy: boolean;
    dialogError: string;
    onsubmit: () => void;
    onclose: () => void;
  } = $props();
</script>

<ResponsiveDialog
  title={kind === "rename" ? m.s5_rename() : m.s5_new_folder()}
  description={namingDialogDescription(kind, node)}
  {onclose}
  {busy}
>
  <form
    class="space-y-4"
    onsubmit={(event) => {
      event.preventDefault();
      onsubmit();
    }}
  >
    <div class="space-y-2">
      <Label for="code-file-name">{m.s5_name_label()}</Label>
      <Input
        id="code-file-name"
        bind:ref={nameInput}
        bind:value={name}
        required
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
    </div>
    {#if dialogError}<p role="alert" class="text-sm break-all text-destructive">
        {dialogError}
      </p>{/if}
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" disabled={busy} onclick={onclose}
        >{m.s5_cancel()}</Button
      >
      <Button type="submit" disabled={busy || !name}
        >{kind === "rename" ? m.s5_rename() : m.s5_create()}</Button
      >
    </div>
  </form>
</ResponsiveDialog>
