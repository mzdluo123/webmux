<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import { api } from "../../platform/api";
  import { NoContentSchema, type Repository } from "../../contracts/schemas";

  let {
    repository,
    open = $bindable(),
    onchange,
  }: {
    // Kept after closing so the closing animation still shows its path.
    repository: Repository | null;
    open: boolean;
    onchange: () => Promise<void>;
  } = $props();

  async function unregister() {
    if (!repository) return;
    await api(`/repositories/${repository.id}`, NoContentSchema, "DELETE");
    open = false;
    await onchange();
  }
</script>

<ConfirmDialog
  bind:open
  title={m.s8_unregister_repo_title()}
  confirmLabel={m.s8_unregister_repo_confirm()}
  busyLabel={m.s8_unregister_repo_busy()}
  onconfirm={unregister}
>
  <span class="break-all whitespace-pre-wrap"
    >{repository?.anchorPath}<br />{m.s8_unregister_repo_body()}</span
  >
</ConfirmDialog>
