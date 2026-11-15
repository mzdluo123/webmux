<script lang="ts">
  import IconNotes from "~icons/tabler/notes";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import * as Popover from "$lib/components/ui/popover";
  import * as m from "$lib/paraglide/messages.js";
  import {
    expandInitInstruction,
    usesArtifactDir,
    parseQuickCommands,
  } from "../../domains/settings/quick-commands";
  import { settings } from "../../domains/settings/settings.svelte";
  import { pluginAvailable } from "../../platform/plugin-host.svelte";
  import type { Repository } from "../../contracts/schemas";

  let {
    compact = false,
    repository,
    sending,
    useInstruction,
    onchange,
  }: {
    compact?: boolean;
    repository: Repository;
    sending: boolean;
    useInstruction: boolean;
    onchange: (use: boolean) => void;
  } = $props();

  const instructionPreview = $derived(
    expandInitInstruction(
      repository.initInstruction ?? "",
      pluginAvailable("quick-commands.send")
        ? (settings.values.quickCommands ?? parseQuickCommands(""))
        : [],
    ),
  );
</script>

<div
  class={[
    "flex items-center gap-2 border-b px-3 py-1",
    compact && "composer-mobile:hidden",
  ]}
>
  <Checkbox
    id="use-init-instruction"
    disabled={sending}
    bind:checked={() => useInstruction, onchange}
  />
  <Label for="use-init-instruction">{m.s3_use_init_instruction()}</Label>
  <Popover.Root>
    <Popover.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="quiet"
          size="xs"
          title={m.s3_qs_init_preview_title()}
          ><IconNotes aria-hidden="true" />{m.s3_preview()}</Button
        >
      {/snippet}
    </Popover.Trigger>
    <Popover.Content align="start" class="w-80 space-y-2">
      <p class="font-medium">
        {m.s3_qs_init_repo_heading({ name: repository.name })}
      </p>
      <p
        class="max-h-64 overflow-y-auto whitespace-pre-wrap break-words text-muted-foreground"
      >
        {instructionPreview.hasUser
          ? instructionPreview.text.replaceAll(
              "{{user}}",
              m.s3_init_user_placeholder(),
            )
          : instructionPreview.text}
      </p>
      {#if instructionPreview.missing.length}
        <p class="text-xs text-warning" role="status">
          {m.s3_qs_init_missing_commands({
            names: instructionPreview.missing
              .map((name) => `{{${name}}}`)
              .join(m.s3_list_separator()),
          })}
        </p>
      {/if}
      {#if !instructionPreview.hasUser}
        <p class="text-xs text-muted-foreground">
          {m.s3_qs_init_no_user_token({ token: "{{user}}" })}
        </p>
      {/if}
      {#if usesArtifactDir(instructionPreview.text)}
        <p class="text-xs text-muted-foreground">
          {m.s3_qs_init_dir_token({ token: "{{dir}}" })}
        </p>
      {/if}
      {#if instructionPreview.hasWebmux}
        <p class="text-xs text-muted-foreground">
          {m.s3_qs_init_webmux_token({ token: "{{webmux}}" })}
        </p>
      {/if}
      <p class="text-xs text-muted-foreground">
        {m.s3_qs_init_footer_hint()}
      </p>
    </Popover.Content>
  </Popover.Root>
</div>
