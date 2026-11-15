<script lang="ts">
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as m from "$lib/paraglide/messages.js";
  import { containsRedactedRemoteMask } from "../../domains/workspace/git-v2-adapter";
  import type { GitRemotesController } from "../../hooks/workspace/git-remotes-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";

  let { remotes }: { remotes: GitRemotesController } = $props();

  const hasMask = $derived(
    containsRedactedRemoteMask(remotes.fetchUrl) ||
      containsRedactedRemoteMask(remotes.pushUrl),
  );
  const canSave = $derived(
    !remotes.submitting &&
      Boolean(remotes.name.trim()) &&
      Boolean(remotes.fetchUrl.trim()) &&
      !hasMask,
  );
</script>

{#if remotes.editOpen}
  <ResponsiveDialog
    title={remotes.editMode === "add"
      ? m.s6_remote_add_title()
      : m.s6_remote_edit_title({ name: remotes.name })}
    description={m.s6_remote_edit_desc()}
    drawerDescription={m.s6_remote_edit_drawer_desc()}
    busy={remotes.submitting}
    class="sm:max-w-lg"
    onclose={() => remotes.closeDialogs()}
  >
    <div class="space-y-3" data-testid="git-remote-edit-dialog">
      <div class="space-y-1.5">
        <Label for="git-remote-name-input">{m.s6_remote_name_label()}</Label>
        <Input
          id="git-remote-name-input"
          mono
          value={remotes.name}
          placeholder="origin"
          disabled={remotes.editMode === "edit" || remotes.submitting}
          oninput={(e) => remotes.setName(e.currentTarget.value)}
          data-testid="git-remote-name-input"
        />
      </div>

      <div class="space-y-1.5">
        <Label for="git-remote-fetch-input"
          >{m.s6_remote_fetch_url_label()}</Label
        >
        <Input
          id="git-remote-fetch-input"
          mono
          value={remotes.fetchUrl}
          placeholder={m.s6_remote_fetch_url_placeholder()}
          disabled={remotes.submitting}
          oninput={(e) => remotes.setFetchUrl(e.currentTarget.value)}
          data-testid="git-remote-fetch-input"
        />
      </div>

      <div class="space-y-1.5">
        <Label for="git-remote-push-input">
          {m.s6_remote_push_url_label()}
        </Label>
        <Input
          id="git-remote-push-input"
          mono
          value={remotes.pushUrl}
          placeholder={m.s6_remote_push_url_placeholder()}
          disabled={remotes.submitting}
          oninput={(e) => remotes.setPushUrl(e.currentTarget.value)}
          data-testid="git-remote-push-input"
        />
      </div>

      {#if hasMask}
        <InlineNotice tone="warning">
          {m.s6_remote_mask_before()}<span class="font-mono">***</span
          >{m.s6_remote_mask_after()}
        </InlineNotice>
      {/if}

      <GitDialogFooter
        dialogError={remotes.dialogError}
        submitting={remotes.submitting}
        canConfirm={canSave}
        confirmLabel={m.s6_remote_save_btn()}
        submittingLabel={m.s6_saving_ellipsis()}
        confirmTestId="git-remote-save-btn"
        onConfirm={() => void remotes.confirmSave()}
        onClose={() => remotes.closeDialogs()}
      />
    </div>
  </ResponsiveDialog>
{/if}

{#if remotes.removeTarget}
  <ResponsiveDialog
    title={m.s6_remote_remove_title({ name: remotes.removeTarget.name })}
    description={m.s6_remote_remove_desc()}
    drawerDescription={m.s6_remote_remove_drawer_desc()}
    busy={remotes.submitting}
    class="sm:max-w-md"
    onclose={() => remotes.closeDialogs()}
  >
    <div class="space-y-3" data-testid="git-remote-remove-dialog">
      <div class="space-y-1 rounded-lg border bg-background p-3 text-xs">
        <p class="font-mono font-semibold text-foreground">
          {remotes.removeTarget.name}
        </p>
        <p class="font-mono break-all text-muted-foreground">
          {remotes.removeTarget.fetchUrl}
        </p>
      </div>

      <GitDialogFooter
        dialogError={remotes.dialogError}
        submitting={remotes.submitting}
        confirmLabel={m.s6_remote_remove_confirm()}
        submittingLabel={m.s6_remote_remove_submitting()}
        confirmVariant="destructive"
        confirmTestId="git-remote-remove-confirm-btn"
        onConfirm={() => void remotes.confirmRemove()}
        onClose={() => remotes.closeDialogs()}
      />
    </div>
  </ResponsiveDialog>
{/if}
