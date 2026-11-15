<script lang="ts">
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as m from "$lib/paraglide/messages.js";

  // The "first message" rows of the new-session dialog: the repository's
  // initialization instruction (when the location has one) and the user's
  // own instruction. Whatever is set is sent once the session is up, with
  // the instruction first: as the first chat prompt, or typed into the
  // terminal followed by Enter.
  let {
    repositoryName = "",
    hasInstruction,
    useInstruction = $bindable(),
    prompt = $bindable(),
    chat,
    disabled = false,
  }: {
    repositoryName?: string;
    hasInstruction: boolean;
    useInstruction: boolean;
    prompt: string;
    /** The session starts on the chat side. */
    chat: boolean;
    disabled?: boolean;
  } = $props();
</script>

<div class="space-y-3">
  {#if hasInstruction}
    <div class="flex items-start gap-2">
      <Checkbox
        id="use-init-instruction"
        class="mt-0.5"
        bind:checked={useInstruction}
        {disabled}
      />
      <div class="grid gap-0.5">
        <Label for="use-init-instruction">{m.s3_use_init_instruction()}</Label>
        <p class="text-xs text-muted-foreground">
          {#if chat}
            {repositoryName
              ? m.s3_first_msg_init_chat_named({ name: repositoryName })
              : m.s3_first_msg_init_chat_generic()}
          {:else}
            {m.s3_first_msg_init_terminal()}
          {/if}
        </p>
      </div>
    </div>
  {/if}
  <div class="space-y-2">
    <Label for="session-prompt"
      >{m.s3_user_instruction()}
      <span class="text-muted-foreground">{m.s3_optional_tag()}</span></Label
    >
    <Textarea
      id="session-prompt"
      rows={3}
      bind:value={prompt}
      {disabled}
      aria-describedby="session-prompt-help"
      placeholder={m.s3_user_instruction_placeholder()}
    />
    <p id="session-prompt-help" class="text-xs text-muted-foreground">
      {chat ? m.s3_first_msg_help_chat() : m.s3_first_msg_help_terminal()}
    </p>
  </div>
</div>
