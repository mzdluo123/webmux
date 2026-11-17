<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { m } from "$lib/paraglide/messages.js";

  // A hidden multi-file picker; the parent opens it from its own button.
  let {
    upload,
    label,
  }: {
    upload: (files: File[]) => Promise<void>;
    label?: string;
  } = $props();
  let input = $state<HTMLInputElement | null>(null);
  const resolvedLabel = $derived(label ?? m.s1_choose_upload_files());

  export function open() {
    input?.click();
  }
</script>

<Input
  bind:ref={input}
  type="file"
  multiple
  class="hidden"
  aria-label={resolvedLabel}
  onchange={(event) => {
    const files = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = "";
    void upload(files);
  }}
/>
