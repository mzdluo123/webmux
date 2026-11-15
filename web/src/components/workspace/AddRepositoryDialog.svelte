<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import { api } from "../../platform/api";
  import {
    RegisterRepositoryRequestSchema,
    RepositorySchema,
  } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { getPluginContributions } from "../../plugins/context";
  const DirectoryPicker = getPluginContributions().creation.DirectoryPicker;
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  let {
    defaultCwd,
    onclose,
    onchange,
  }: { defaultCwd: string; onclose: () => void; onchange: () => void } =
    $props();
  let path = $state("");
  let busy = $state(false);
  let error = $state("");
  async function add() {
    busy = true;
    error = "";
    try {
      await api(
        "/repositories",
        RepositorySchema,
        "POST",
        { path },
        undefined,
        RegisterRepositoryRequestSchema,
      );
      onchange();
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<ResponsiveDialog
  title={m.s8_add_repo_title()}
  description={m.s8_add_repo_desc()}
  {onclose}
  {busy}
>
  <form
    class="space-y-4"
    onsubmit={(event) => {
      event.preventDefault();
      void add();
    }}
  >
    <div class="space-y-2">
      <Label for="repository-path">{m.s8_add_repo_dir_label()}</Label>
      <div class="flex gap-2">
        <Input
          id="repository-path"
          bind:value={path}
          placeholder={defaultCwd}
          required
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck={false}
        />
        {#if DirectoryPicker}<DirectoryPicker bind:value={path} />{/if}
      </div>
    </div>
    <p class="text-sm text-muted-foreground">
      {m.s8_add_repo_not_git_hint()}
    </p>
    {#if error}<p role="alert" class="break-all text-sm text-destructive">
        {error}
      </p>{/if}
    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline" disabled={busy} onclick={onclose}
        >{m.s8_cancel()}</Button
      >
      <Button type="submit" disabled={busy || !path}
        >{busy ? m.s8_adding_ellipsis() : m.s8_add_repo_title()}</Button
      >
    </div>
  </form>
</ResponsiveDialog>
