<script lang="ts">
  import type { Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import SessionPanelPickerSheet from "./SessionPanelPickerSheet.svelte";
  import SessionSidePanel from "./SessionSidePanel.svelte";

  // Hosts the side panel and its picker: a page inside the panel on
  // desktop, a bottom sheet on phones, so `view` never takes "picker" on a
  // phone.
  let {
    view = $bindable("code"),
    shown = $bindable(false),
    codeAvailable = true,
    close,
    paneActions,
    children,
  }: {
    view?: "picker" | "code" | "split";
    shown?: boolean;
    codeAvailable?: boolean;
    close: () => void;
    paneActions?: Snippet<[() => void]>;
    children: Snippet;
  } = $props();

  let sidePanel = $state<SessionSidePanel>();
  const phone = new MediaQuery("(max-width: 639px)");
  let pickerOpen = $state(false);
  // The picker page is a desktop surface; on phones `view` falls back to
  // the last real panel so a resize or a restored state can never leave
  // the page and the sheet mounted at once.
  let lastView = $state<"code" | "split">("code");

  $effect(() => {
    if (view === "picker") {
      if (phone.current) view = lastView;
    } else lastView = view;
  });

  export function openPanels() {
    if (phone.current) {
      if (view === "picker") view = lastView;
      pickerOpen = true;
      return;
    }
    shown = true;
    sidePanel?.openPicker();
  }

  function choose(next: "code" | "split") {
    view = next;
    shown = true;
  }
</script>

<SessionSidePanel
  bind:this={sidePanel}
  bind:view
  {shown}
  {codeAvailable}
  {close}
  onpicker={openPanels}
  {paneActions}
>
  {@render children()}
</SessionSidePanel>
<SessionPanelPickerSheet
  bind:open={pickerOpen}
  {codeAvailable}
  splitsAvailable={!!paneActions}
  {shown}
  current={shown && view !== "picker" ? view : undefined}
  {choose}
  closePanel={close}
/>
