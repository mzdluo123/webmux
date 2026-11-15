<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import {
    settings,
    changeSettings,
  } from "../../domains/settings/settings.svelte";
  import { defaultCodeReader } from "../../domains/settings/settings-intent";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";

  const code = $derived(settings.values.codeReader ?? defaultCodeReader);
</script>

<Card.Root flat id="code-reader" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_code_reader_title()}</Card.Title>
    <Card.Description>{m.s2_code_reader_desc()}</Card.Description>
  </Card.Header>
  <Card.Content>
    <fieldset disabled={!settings.ready} class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <Label for="settings-lines">{m.s2_code_line_numbers()}</Label>
        <Switch
          id="settings-lines"
          checked={code.lineNumbers === "show"}
          onCheckedChange={(value) =>
            changeSettings({
              codeReader: { ...code, lineNumbers: value ? "show" : "hide" },
            })}
        />
      </div>
      <div class="flex items-center justify-between gap-3">
        <Label for="settings-wrap">{m.s2_code_diff_wrap()}</Label>
        <Switch
          id="settings-wrap"
          checked={code.diffOverflow === "wrap"}
          onCheckedChange={(value) =>
            changeSettings({
              codeReader: { ...code, diffOverflow: value ? "wrap" : "scroll" },
            })}
        />
      </div>
      <Button
        variant="outline"
        size="sm"
        onclick={() => changeSettings({ codeReader: null })}
        >{m.s2_code_reset_defaults()}</Button
      >
    </fieldset>
  </Card.Content>
</Card.Root>
