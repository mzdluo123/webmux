<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconCode from "~icons/tabler/code";
  import IconLayoutColumns from "~icons/tabler/layout-columns";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import { m } from "$lib/paraglide/messages.js";

  // The phone form of the side panel's picker: a bottom sheet instead of a
  // page inside the panel, so the terminal or current panel stays behind
  // the scrim and dismissing the sheet changes nothing.
  let {
    open = $bindable(false),
    codeAvailable = true,
    splitsAvailable = true,
    /** The side panel is open; the sheet then offers its close action. */
    shown = false,
    /** The panel view currently behind the sheet, marked as current. */
    current,
    choose,
    closePanel,
  }: {
    open?: boolean;
    codeAvailable?: boolean;
    splitsAvailable?: boolean;
    shown?: boolean;
    current?: "code" | "split";
    choose: (view: "code" | "split") => void;
    closePanel: () => void;
  } = $props();

  function pick(view: "code" | "split") {
    open = false;
    choose(view);
  }
</script>

<Drawer.Root bind:open>
  <Drawer.Content>
    <Drawer.Header>
      <Drawer.Title>{m.s4_open_panel()}</Drawer.Title>
      <Drawer.Description>{m.s4_panel_picker_hint()}</Drawer.Description>
    </Drawer.Header>
    <div
      class="flex min-h-0 flex-col overflow-y-auto px-2 pb-safe-4"
      role="group"
      aria-label={m.s4_panel_picker()}
    >
      {#if codeAvailable}
        <ListButton
          variant="action-sheet"
          aria-current={current === "code" ? "true" : undefined}
          onclick={() => pick("code")}
        >
          <IconCode aria-hidden="true" /><span class="min-w-0 flex-1"
            >{m.s4_code()}</span
          >
          {#if current === "code"}<IconCheck
              class="text-primary"
              aria-hidden="true"
            />{/if}
        </ListButton>
      {/if}
      {#if splitsAvailable}
        <ListButton
          variant="action-sheet"
          aria-current={current === "split" ? "true" : undefined}
          onclick={() => pick("split")}
        >
          <IconLayoutColumns aria-hidden="true" /><span class="min-w-0 flex-1"
            >{m.s4_split_pane()}</span
          >
          {#if current === "split"}<IconCheck
              class="text-primary"
              aria-hidden="true"
            />{/if}
        </ListButton>
      {/if}
      {#if shown}
        <div class="px-3 py-1"><Separator /></div>
        <ListButton
          variant="action-sheet"
          onclick={() => {
            open = false;
            closePanel();
          }}
        >
          <IconX aria-hidden="true" /><span class="min-w-0 flex-1"
            >{m.s4_close_side_panel()}</span
          >
        </ListButton>
      {/if}
      <Button
        variant="outline"
        class="mt-2 w-full"
        onclick={() => (open = false)}>{m.s4_cancel()}</Button
      >
    </div>
  </Drawer.Content>
</Drawer.Root>
