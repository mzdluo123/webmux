<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import DirectoryPicker from "./DirectoryPicker.svelte";
  let {
    label,
    id,
    value = $bindable(""),
    disabled = false,
  }: {
    label: string;
    id: string;
    value: string;
    disabled?: boolean;
  } = $props();
</script>

<div class="space-y-2">
  <Label for={id}>{label}</Label>
  <div class="flex min-w-0 gap-2">
    <Input
      {id}
      bind:value
      {disabled}
      mono
      required
      autocomplete="off"
      autocapitalize="off"
      spellcheck={false}
    />
    <fieldset {disabled}>
      <DirectoryPicker bind:value endpoint="/projects/directories" />
    </fieldset>
  </div>
</div>
