<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as m from "$lib/paraglide/messages.js";

  let {
    command = $bindable(""),
    showLabel = false,
    disabled = false,
    oninput,
  }: {
    command: string;
    showLabel?: boolean;
    disabled?: boolean;
    oninput?: () => void;
  } = $props();
</script>

<div class="space-y-2">
  {#if showLabel}<Label for="custom-command">{m.s3_custom_launch_cmd()}</Label
    >{/if}
  <Input
    id="custom-command"
    aria-label={m.s3_custom_launch_cmd()}
    aria-describedby="custom-command-help"
    bind:value={command}
    {disabled}
    {oninput}
    mono
    placeholder={m.s3_custom_cmd_placeholder()}
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
  />
  <p id="custom-command-help" class="text-xs text-muted-foreground">
    {m.s3_custom_cmd_help()}
  </p>
</div>
