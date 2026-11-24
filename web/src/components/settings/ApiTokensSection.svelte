<script lang="ts">
  import IconCopy from "~icons/tabler/copy";
  import IconKey from "~icons/tabler/key";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Input } from "$lib/components/ui/input";
  import type { AuthAPIToken } from "../../contracts/auth-schemas";
  import { createAPIToken, revokeAPIToken } from "../../platform/auth-api";
  import { copyText } from "../../platform/clipboard";
  import { errorMessage } from "../../platform/error";
  import { m } from "$lib/paraglide/messages.js";

  let {
    tokens,
    busy,
    formatDate,
    onRefresh,
    onError,
  }: {
    tokens: AuthAPIToken[];
    busy: boolean;
    formatDate: (iso?: string) => string;
    onRefresh: () => Promise<void>;
    onError: (msg: string) => void;
  } = $props();

  let newTokenName = $state("");
  let createdToken = $state("");
  let submitting = $state(false);

  async function handleCreateToken() {
    if (!newTokenName.trim()) return;
    submitting = true;
    onError("");
    try {
      const created = await createAPIToken(newTokenName.trim(), 90);
      createdToken = created.token;
      newTokenName = "";
      await onRefresh();
    } catch (err) {
      onError(errorMessage(err));
    } finally {
      submitting = false;
    }
  }

  async function handleRevokeToken(id: string) {
    submitting = true;
    onError("");
    try {
      await revokeAPIToken(id);
      await onRefresh();
    } catch (err) {
      onError(errorMessage(err));
    } finally {
      submitting = false;
    }
  }
</script>

<div class="space-y-3">
  <div class="space-y-0.5">
    <h3 class="text-sm font-medium flex items-center gap-1.5">
      <IconKey class="size-4" aria-hidden="true" />
      {m.settings_tokens_heading()}
    </h3>
    <p class="text-xs text-muted-foreground">{m.settings_tokens_desc()}</p>
  </div>
  <form
    class="flex flex-col gap-2 sm:flex-row"
    onsubmit={(event) => {
      event.preventDefault();
      void handleCreateToken();
    }}
  >
    <Input
      type="text"
      bind:value={newTokenName}
      placeholder={m.settings_token_name_placeholder()}
      disabled={busy || submitting}
      class="flex-1"
    />
    <Button
      type="submit"
      variant="outline"
      size="sm"
      disabled={busy || submitting || !newTokenName.trim()}
    >
      <IconPlus class="size-4" aria-hidden="true" />
      {m.settings_token_create()}
    </Button>
  </form>
  {#if createdToken}
    <InlineNotice tone="warning">
      <div class="space-y-2">
        <p class="text-xs">{m.settings_token_created_notice()}</p>
        <div class="flex flex-wrap items-center gap-2">
          <code class="break-all text-xs font-mono select-all flex-1">
            {createdToken}
          </code>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onclick={() => void copyText(createdToken)}
          >
            <IconCopy class="size-4" aria-hidden="true" />
            {m.settings_pair_copy_code()}
          </Button>
        </div>
      </div>
    </InlineNotice>
  {/if}
  {#if tokens.length === 0}
    <p class="text-xs text-muted-foreground">{m.settings_token_empty()}</p>
  {:else}
    <div class="divide-y rounded-lg border">
      {#each tokens as tok (tok.id)}
        <div class="flex flex-wrap items-center justify-between gap-3 p-3">
          <div class="min-w-0 space-y-0.5">
            <span class="text-sm font-medium">{tok.name}</span>
            <p class="text-xs text-muted-foreground">
              {formatDate(tok.createdAt)}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={busy || submitting}
            onclick={() => void handleRevokeToken(tok.id)}
          >
            {m.settings_token_revoke()}
          </Button>
        </div>
      {/each}
    </div>
  {/if}
</div>
