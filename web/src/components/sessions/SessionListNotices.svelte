<script lang="ts">
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";

  let {
    setupError,
    setupBusy,
    onretrysetup,
    error,
    rebuildFailure,
    recordError,
    capabilityError,
    repositoryError,
    hasRepositories,
  }: {
    setupError: string;
    setupBusy: boolean;
    onretrysetup: () => void;
    error: string;
    rebuildFailure: string;
    recordError: string;
    capabilityError: string;
    repositoryError: string;
    hasRepositories: boolean;
  } = $props();
</script>

{#if setupError}<InlineNotice tone="warning" class="mb-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span>{setupError}</span>
      <Button
        variant="ghost"
        size="sm"
        class="min-h-9 sm:min-h-8"
        disabled={setupBusy}
        onclick={onretrysetup}
      >
        {setupBusy ? m.s3_retrying() : m.s3_retry()}
      </Button>
    </div>
  </InlineNotice>{/if}
{#if error}<InlineNotice tone="danger" class="mb-4">
    {error}
  </InlineNotice>{/if}
{#if rebuildFailure}<InlineNotice tone="danger" class="mb-4">
    {rebuildFailure}
  </InlineNotice>{/if}
{#if recordError}<InlineNotice tone="danger" class="mb-4">
    {recordError}
  </InlineNotice>{/if}
{#if capabilityError}<InlineNotice tone="warning" class="mb-4">
    {m.s3_repo_mgmt_partially_unavailable({ error: capabilityError })}
  </InlineNotice>{/if}
{#if repositoryError}<InlineNotice tone="danger" class="mb-4">
    {hasRepositories
      ? m.s3_git_refresh_failed_keep_snapshot({ error: repositoryError })
      : m.s3_git_read_failed_sessions_ok({ error: repositoryError })}
  </InlineNotice>{/if}
