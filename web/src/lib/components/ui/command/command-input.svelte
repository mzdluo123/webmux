<script lang="ts">
  import { Command as CommandPrimitive } from "bits-ui";
  import * as InputGroup from "$lib/components/ui/input-group/index.js";
  import IconSearch from "~icons/tabler/search";

  let {
    ref = $bindable(null),
    class: className,
    value = $bindable(""),
    ...restProps
  }: CommandPrimitive.InputProps = $props();
</script>

<InputGroup.Root variant="list-search" data-slot="command-input-wrapper">
  <CommandPrimitive.Input
    {value}
    data-slot="command-input"
    class={className}
    {...restProps}
  >
    {#snippet child({ props })}
      <InputGroup.Input {...props} bind:value bind:ref />
    {/snippet}
  </CommandPrimitive.Input>
  <InputGroup.Addon><IconSearch aria-hidden="true" /></InputGroup.Addon>
</InputGroup.Root>
