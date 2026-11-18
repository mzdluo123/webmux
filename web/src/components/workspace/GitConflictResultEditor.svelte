<script lang="ts">
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";

  let {
    ref = $bindable(null),
    value,
    oninput,
    mobile = false,
  }: {
    ref?: HTMLTextAreaElement | null;
    value: string;
    oninput: (value: string) => void;
    mobile?: boolean;
  } = $props();
</script>

<Textarea
  variant="bare"
  code
  aria-label={m.s6_conflict_result_editor_aria()}
  bind:ref
  {value}
  oninput={(event) => oninput(event.currentTarget.value)}
  class={mobile ? "min-h-40" : "min-h-full flex-1"}
  data-testid={mobile
    ? "git-conflict-mobile-textarea"
    : "git-conflict-result-textarea"}
/>
