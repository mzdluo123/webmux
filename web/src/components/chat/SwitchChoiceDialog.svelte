<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as m from "$lib/paraglide/messages.js";

  // D21: the interrupt key did not stop the terminal's turn within 10
  // seconds. Every device sees the question; the first answer applies.
  let {
    open = $bindable(false),
    note,
    busy,
    onchoose,
  }: {
    open?: boolean;
    note: string;
    busy: boolean;
    onchoose: (choice: "wait" | "force" | "cancel") => void;
  } = $props();
</script>

<AlertDialog.Root bind:open>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s3_switch_choice_title()}</AlertDialog.Title>
      <AlertDialog.Description>
        {m.s3_switch_choice_desc()}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <ul class="space-y-2 text-sm text-muted-foreground">
      <li>
        <strong class="text-foreground">{m.s3_keep_waiting()}</strong
        >{m.s3_switch_choice_wait_detail()}
      </li>
      <li>
        <strong class="text-foreground">{m.s3_force_switch()}</strong>{note
          ? m.s3_switch_choice_force_detail_note({ note })
          : m.s3_switch_choice_force_detail()}
      </li>
      <li>
        <strong class="text-foreground">{m.s3_cancel()}</strong
        >{m.s3_switch_choice_cancel_detail()}
      </li>
    </ul>
    <AlertDialog.Footer>
      <Button
        variant="outline"
        disabled={busy}
        onclick={() => onchoose("cancel")}>{m.s3_cancel()}</Button
      >
      <Button
        variant="destructive"
        disabled={busy}
        onclick={() => onchoose("force")}>{m.s3_force_switch()}</Button
      >
      <Button disabled={busy} onclick={() => onchoose("wait")}
        >{m.s3_keep_waiting()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
