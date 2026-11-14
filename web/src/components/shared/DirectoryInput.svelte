<script lang="ts">
  import type { Component } from "svelte";
  import { Input } from "$lib/components/ui/input";

  let {
    value = $bindable(""),
    ref = $bindable<HTMLInputElement | null>(null),
    placeholder,
    Picker,
  }: {
    value: string;
    ref?: HTMLInputElement | null;
    placeholder?: string;
    Picker?: Component<{ value: string }, {}, "value">;
  } = $props();
</script>

<div class="flex gap-2">
  <Input
    id="cwd"
    aria-label="工作目录"
    bind:ref
    bind:value
    {placeholder}
    mono
    required
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
  />
  {#if Picker}<Picker bind:value />{/if}
</div>
