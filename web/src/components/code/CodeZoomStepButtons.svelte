<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import IconZoomIn from "~icons/tabler/zoom-in";
  import IconZoomOut from "~icons/tabler/zoom-out";

  let {
    canZoomOut,
    canZoomIn,
    onzoomout,
    onzoomin,
  }: {
    canZoomOut: boolean;
    canZoomIn: boolean;
    onzoomout: () => void;
    onzoomin: () => void;
  } = $props();
</script>

<!-- Narrow touch screens zoom by pinching; keep the row to one line. -->
<Button
  variant="outline"
  size="icon-sm"
  class="max-sm:any-pointer-coarse:hidden"
  aria-label={m.s5_zoom_out()}
  title={m.s5_zoom_out()}
  disabled={!canZoomOut}
  onclick={onzoomout}><IconZoomOut aria-hidden="true" /></Button
>
<Button
  variant="outline"
  size="icon-sm"
  class="max-sm:any-pointer-coarse:hidden"
  aria-label={m.s5_zoom_in()}
  title={m.s5_zoom_in()}
  disabled={!canZoomIn}
  onclick={onzoomin}><IconZoomIn aria-hidden="true" /></Button
>
