<script lang="ts">
  import * as Popover from "$lib/components/ui/popover";
  import { m } from "$lib/paraglide/messages.js";
</script>

<Popover.Root>
  <Popover.Trigger
    openOnHover
    openDelay={100}
    closeDelay={150}
    type="button"
    aria-label={m.s1_token_help_trigger_aria()}
    class="inline-flex size-7 shrink-0 cursor-help items-center justify-center rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
  >
    <span
      aria-hidden="true"
      class="inline-flex size-4 items-center justify-center rounded-full border border-current text-xs font-semibold"
      >?</span
    >
  </Popover.Trigger>
  <Popover.Content
    role="dialog"
    aria-label={m.s1_token_help_dialog_aria()}
    align="start"
    collisionPadding={16}
    class="w-[min(20rem,calc(100vw-2rem))] space-y-2"
    trapFocus={false}
    onOpenAutoFocus={(event) => event.preventDefault()}
    onCloseAutoFocus={(event) => event.preventDefault()}
  >
    <p class="leading-relaxed">
      {m.s1_token_help_intro()}
    </p>
    <p class="leading-relaxed">
      {m.s1_token_help_p2_before_init()}<code>webmux config init</code>
      {m.s1_token_help_p2_after_init()}
      <code class="block max-w-full break-all"
        >~/.config/webmux/config.toml</code
      >
      {m.s1_token_help_p2_before_token()}<code>token</code
      >{m.s1_token_help_p2_after_token()}
    </p>
    <p class="leading-relaxed">
      {m.s1_token_help_p3_before_path()}<code>webmux config path</code
      >{m.s1_token_help_p3_after_path()}<code>--config</code
      >{m.s1_token_help_p3_or()}<code>--token</code
      >{m.s1_token_help_p3_after_flags()}
    </p>
  </Popover.Content>
</Popover.Root>
