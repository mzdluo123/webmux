<script lang="ts">
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { m } from "$lib/paraglide/messages.js";
  import { homeSessionView } from "../../domains/sessions/home-session-view.svelte";

  const previewId = $props.id();
</script>

<div class="flex min-h-9 max-w-1/2 shrink-0 items-center gap-2">
  <Switch
    id={previewId}
    checked={!homeSessionView.compact}
    onCheckedChange={(checked) => (homeSessionView.compact = !checked)}
  />
  <Label for={previewId}>{m.s8_show_output_preview()}</Label>
</div>
