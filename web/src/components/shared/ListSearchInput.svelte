<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import * as InputGroup from "$lib/components/ui/input-group";

  let {
    ref = $bindable(null),
    value = $bindable(""),
    label,
    placeholder,
    clearable = false,
  }: {
    ref?: HTMLInputElement | null;
    value?: string;
    label: string;
    placeholder: string;
    clearable?: boolean;
  } = $props();
</script>

<InputGroup.Root variant="list-search">
  <InputGroup.Addon>
    <IconSearch aria-hidden="true" />
  </InputGroup.Addon>
  <InputGroup.Input
    type={clearable ? "search" : "text"}
    aria-label={label}
    {placeholder}
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
    bind:ref
    bind:value
    onkeydown={(event) => {
      if (clearable && event.key === "Escape" && value && !event.isComposing) {
        event.preventDefault();
        event.stopPropagation();
        value = "";
      }
    }}
  />
</InputGroup.Root>
