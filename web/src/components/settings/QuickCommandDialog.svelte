<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { untrack } from "svelte";
  import IconBolt from "~icons/tabler/bolt";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Dialog from "$lib/components/ui/dialog";
  import {
    QuickCommandSchema,
    type QuickCommand,
  } from "../../contracts/schemas";
  import {
    QUICK_COMMAND_NAME_MAX,
    newQuickCommandId,
    normalizeCommand,
    quickCommandNameError,
  } from "../../domains/settings/quick-commands";

  let {
    open = $bindable(false),
    initial,
    save,
    existing,
    disabled = false,
  }: {
    open: boolean;
    disabled?: boolean;
    /** Command being edited; undefined creates a new one. */
    initial: QuickCommand | undefined;
    save: (cmd: QuickCommand) => string | Promise<string>;
    /** The saved list, for the unique-name check. */
    existing: QuickCommand[];
  } = $props();
  let name = $state("");
  let command = $state("");
  let enter = $state(false);
  let error = $state("");
  let saving = $state(false);
  let commandInput = $state<HTMLTextAreaElement | null>(null);
  const draft = $derived(
    QuickCommandSchema.safeParse({
      id: initial?.id ?? "pending",
      name: name.trim(),
      command: normalizeCommand(command),
      enter,
    }),
  );
  const nameError = $derived(
    quickCommandNameError(
      name,
      existing.filter((cmd) => cmd.id !== initial?.id).map((cmd) => cmd.name),
    ),
  );
  // An untouched new form is not an error yet; a saved empty name is.
  const showNameError = $derived(!!nameError && (name !== "" || !!initial));

  // Each opening starts from the saved value, so cancel/Esc discards edits.
  $effect.pre(() => {
    if (!open) return;
    untrack(() => {
      name = initial?.name ?? "";
      command = initial?.command ?? "";
      enter = initial?.enter ?? false;
      error = "";
    });
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!draft.success || nameError) return;
    saving = true;
    try {
      error = await save({
        ...draft.data,
        id: initial?.id ?? newQuickCommandId(),
      });
      if (!error) open = false;
    } catch (err) {
      error = (err as Error).message;
    } finally {
      saving = false;
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    class="sm:max-w-lg"
    onOpenAutoFocus={(event) => {
      if (initial) return;
      event.preventDefault();
      commandInput?.focus();
    }}
  >
    <form class="grid gap-4" onsubmit={submit}>
      <Dialog.Header>
        <Dialog.Title
          >{initial ? m.s2_qc_edit_title() : m.s2_qc_add_title()}</Dialog.Title
        >
        <Dialog.Description
          >{m.s2_qc_dialog_desc_1()}<IconBolt
            class="inline size-3.5 align-[-2px]"
            aria-label={m.s2_bolt_icon_aria()}
          />{m.s2_qc_dialog_desc_2()}</Dialog.Description
        >
      </Dialog.Header>
      <div class="grid gap-2">
        <Label for="quick-command-name">{m.s2_name()}</Label>
        <Input
          id="quick-command-name"
          bind:value={name}
          maxlength={QUICK_COMMAND_NAME_MAX}
          placeholder={m.s2_qc_name_placeholder()}
          autocomplete="off"
          aria-invalid={showNameError}
          aria-describedby="quick-command-name-help"
        />
        <p
          id="quick-command-name-help"
          class={[
            "text-xs",
            showNameError ? "text-destructive" : "text-muted-foreground",
          ]}
        >
          {showNameError
            ? nameError
            : m.s2_qc_name_help({ token: `{{${m.s2_qc_name_token()}}}` })}
        </p>
      </div>
      <div class="grid gap-2">
        <Label for="quick-command-body">{m.s2_qc_body_label()}</Label>
        <Textarea
          id="quick-command-body"
          bind:ref={commandInput}
          bind:value={command}
          rows={4}
          maxlength={4096}
          placeholder={m.s2_qc_body_placeholder()}
          mono
          class="max-h-[40dvh]"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
      </div>
      <div class="flex items-start gap-3">
        <Switch
          id="quick-command-enter"
          bind:checked={enter}
          aria-describedby="quick-command-enter-help"
        />
        <div class="grid gap-1">
          <Label for="quick-command-enter">{m.s2_qc_auto_enter()}</Label>
          <p
            id="quick-command-enter-help"
            class="text-xs text-muted-foreground"
          >
            {m.s2_qc_auto_enter_help()}
          </p>
        </div>
      </div>
      {#if error}
        <InlineNotice tone="danger">{error}</InlineNotice>
      {/if}
      <Dialog.Footer>
        <Button
          type="button"
          variant="outline"
          onclick={() => {
            open = false;
          }}>{m.s2_cancel()}</Button
        >
        <Button
          type="submit"
          disabled={!draft.success || !!nameError || saving || disabled}
          >{m.s2_save()}</Button
        >
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>
