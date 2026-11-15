<script lang="ts">
  import { tick } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";
  import {
    commandLabel,
    DIR_REFERENCE_NAME,
    initInstructionReferences,
    USER_REFERENCE_NAME,
    WEBMUX_REFERENCE_NAME,
  } from "../../domains/settings/quick-commands";
  import {
    INIT_INSTRUCTION_MAX,
    type QuickCommand,
  } from "../../contracts/schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  // The repository init instruction: a template quick start sends before the
  // first message. `{{name}}` references a quick command; the counter turns
  // red past the server limit.
  let {
    value = $bindable(),
    id,
    commands,
    commandsAvailable = true,
    label: labelProp,
  }: {
    value: string;
    id: string;
    commands: QuickCommand[];
    commandsAvailable?: boolean;
    label?: string;
  } = $props();
  const label = $derived(labelProp ?? m.s3_init_instruction_label());
  let field = $state<HTMLTextAreaElement | null>(null);
  const over = $derived(value.length > INIT_INSTRUCTION_MAX);
  const items = $derived(
    commands.reduce<{ name: string; label: string }[]>((acc, cmd) => {
      const name = cmd.name.trim();
      if (name && !acc.some((item) => item.name === name)) {
        acc.push({ name, label: commandLabel(cmd) });
      }
      return acc;
    }, []),
  );
  const unknown = $derived(
    initInstructionReferences(value).filter(
      (name) =>
        name !== USER_REFERENCE_NAME &&
        name !== DIR_REFERENCE_NAME &&
        name !== WEBMUX_REFERENCE_NAME &&
        !items.some((item) => item.name === name),
    ),
  );

  // Inserts at the caret (replacing a selection), or at the end when the
  // field has never been focused.
  async function insert(name: string) {
    const token = `{{${name}}}`;
    const start = field?.selectionStart ?? value.length;
    const end = field?.selectionEnd ?? value.length;
    value = value.slice(0, start) + token + value.slice(end);
    await tick();
    field?.focus();
    field?.setSelectionRange(start + token.length, start + token.length);
  }
</script>

<div class="space-y-2">
  <Label for={id}>{label}</Label>
  <p id={`${id}-help`} class="text-xs text-muted-foreground">
    {m.s3_init_help_intro()}
    {#if commandsAvailable}{m.s3_init_help_cmd_ref({
        token: `{{${m.s3_init_help_cmd_token_name()}}}`,
      })}{/if}
    {m.s3_init_help_write_dir()} <code>{"{{dir}}"}</code>
    {m.s3_init_help_dir_to_webmux()} <code>{"{{webmux}}"}</code>
    {m.s3_init_help_webmux_to_user()}
    <code>{"{{user}}"}</code>
    {m.s3_init_help_outro()}
  </p>
  <Textarea
    {id}
    rows={4}
    bind:ref={field}
    bind:value
    aria-invalid={over}
    aria-describedby={`${id}-help ${id}-count`}
    placeholder={m.s3_init_instruction_placeholder()}
  />
  <p id={`${id}-count`} class="flex justify-end text-xs text-muted-foreground">
    <span class={`tabular-nums ${over ? "text-destructive" : ""}`}
      >{value.length} / {INIT_INSTRUCTION_MAX}</span
    >
  </p>
  <div class="space-y-1">
    <p class="text-xs text-muted-foreground">{m.s3_init_insertable()}</p>
    <div
      class="flex flex-wrap gap-1"
      role="group"
      aria-label={m.s3_quick_commands_label()}
    >
      <Button
        type="button"
        variant="outline"
        size="xs"
        title={m.s3_init_insert_user_title({ token: "{{user}}" })}
        aria-label={m.s3_init_insert_user_label()}
        onclick={() => void insert(USER_REFERENCE_NAME)}
        >{m.s3_init_user_btn()}</Button
      >
      <Button
        type="button"
        variant="outline"
        size="xs"
        title={m.s3_init_insert_webmux_title({ token: "{{webmux}}" })}
        aria-label={m.s3_init_insert_webmux_label()}
        onclick={() => void insert(WEBMUX_REFERENCE_NAME)}
        >{m.s3_init_webmux_btn()}</Button
      >
      {#each items as item (item.name)}
        <Button
          type="button"
          variant="outline"
          size="xs"
          title={m.s3_init_insert_token({ token: `{{${item.name}}}` })}
          aria-label={m.s3_init_insert_named({ name: item.label })}
          onclick={() => void insert(item.name)}>{item.label}</Button
        >
      {/each}
    </div>
    {#if !commandsAvailable}
      <p class="text-xs text-muted-foreground">
        {m.s3_init_commands_unavailable()}
      </p>
    {:else if !items.length}
      <p class="text-xs text-muted-foreground">
        {m.s3_init_no_named_commands()}
      </p>
    {/if}
  </div>
  {#if unknown.length}
    <InlineNotice tone="warning">
      <p>
        {m.s3_init_unknown_commands_warning({
          names: unknown
            .map((name) => `{{${name}}}`)
            .join(m.s3_list_separator()),
        })}
      </p>
    </InlineNotice>
  {/if}
</div>
