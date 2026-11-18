<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { EnvRow } from "../../domains/settings/launch-configs";
  import type {
    VoiceSettings,
    VoiceSettingsUpdate,
  } from "../../contracts/schemas";
  import EnvRowsEditor from "./EnvRowsEditor.svelte";
  import { useLayoutRowsMotion } from "../../hooks/shared/layout-rows-motion.svelte";

  let {
    current,
    disabled,
    onsave,
    onreset,
  }: {
    current: VoiceSettings;
    disabled: boolean;
    onsave: (draft: VoiceSettingsUpdate) => void;
    onreset: () => void;
  } = $props();

  const envName = /^[A-Za-z_][A-Za-z0-9_]*$/;
  let nextKey = 0;
  const makeRow = (name = "", value = ""): EnvRow => ({
    key: nextKey++,
    name,
    value,
  });
  // The command is the user's own: WebMux ships no recognition service.
  // One argument per line: the command runs as an argv, never through a shell.
  let command = $state("");
  let rows = $state<EnvRow[]>([]);
  let attempted = $state(false);
  let actions = $state<HTMLDivElement | null>(null);
  let saveButton = $state<HTMLButtonElement | null>(null);
  let resetButton = $state<HTMLButtonElement | null>(null);
  const webSource = $derived(current.source === "web");

  useLayoutRowsMotion(
    () => actions,
    () => current.source,
    "[data-layout-row]",
  );

  $effect.pre(() => {
    command = current.cmd.join("\n");
    rows = Object.entries(current.env).map(([name, value]) =>
      makeRow(name, value),
    );
    attempted = false;
    if (!webSource && resetButton && document.activeElement === resetButton)
      saveButton?.focus({ preventScroll: true });
  });

  const argv = $derived(
    command
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean),
  );
  // A row left empty is ignored, like in launch configs.
  const filled = $derived(rows.filter((row) => row.name.trim() || row.value));
  const problems = $derived.by(() => {
    const found: Record<number, { name: string }> = {};
    for (const [index, row] of filled.entries()) {
      const name = row.name.trim();
      if (!envName.test(name))
        found[row.key] = {
          name: m.s2_voice_err_env_invalid(),
        };
      else if (
        filled.findIndex((other) => other.name.trim() === name) !== index
      )
        found[row.key] = { name: m.s2_env_err_duplicate() };
    }
    return found;
  });
  // Problems show once the user tried to save.
  const errors = $derived(attempted ? problems : {});
  const invalid = $derived(Object.keys(problems).length > 0);

  function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (disabled || invalid) return;
    onsave({
      cmd: argv,
      env: Object.fromEntries(
        filled.map((row) => [row.name.trim(), row.value]),
      ),
    });
  }
</script>

<form onsubmit={submit} class="space-y-4" aria-label={m.s2_voice_form_aria()}>
  <div class="space-y-1.5">
    <Label for="voice-command">{m.s2_voice_cmd_label()}</Label>
    <Textarea
      id="voice-command"
      code
      rows={4}
      bind:value={command}
      {disabled}
      placeholder="node&#10;/path/to/your-asr-provider.js"
      aria-describedby="voice-command-help"
    />
    <p id="voice-command-help" class="text-xs text-muted-foreground">
      {m.s2_voice_cmd_help()}
    </p>
  </div>
  <EnvRowsEditor bind:rows {errors} {disabled} {makeRow}>
    {#snippet help()}{m.s2_voice_env_help()}{/snippet}
  </EnvRowsEditor>
  <div class="flex flex-wrap gap-2" bind:this={actions}>
    <Button
      type="submit"
      bind:ref={saveButton}
      aria-disabled={disabled}
      data-layout-row="save">{m.s2_save()}</Button
    >
    <div
      class:invisible={!webSource}
      inert={!webSource}
      data-layout-row={webSource ? "reset" : undefined}
    >
      <Button
        type="button"
        variant="outline"
        wrap
        bind:ref={resetButton}
        aria-disabled={disabled}
        onclick={() => {
          if (!disabled) onreset();
        }}>{m.s2_voice_reset_config()}</Button
      >
    </div>
  </div>
</form>
