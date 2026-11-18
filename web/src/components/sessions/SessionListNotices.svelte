<script lang="ts">
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
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
  const hasNotices = $derived(
    !!(
      setupError ||
      error ||
      rebuildFailure ||
      recordError ||
      capabilityError ||
      repositoryError
    ),
  );
</script>

<ScrollArea
  class="h-24"
  type="auto"
  viewportClasses="overscroll-y-contain"
  viewportProps={{
    role: "region",
    "aria-label": m.s1_home_list_notices(),
    "aria-hidden": !hasNotices,
    tabindex: hasNotices ? 0 : undefined,
  }}
>
  <div class="space-y-2 pr-3 pb-2">
    {#if setupError}<InlineNotice tone="warning">
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
    {#if error}<InlineNotice tone="danger">
        {error}
      </InlineNotice>{/if}
    {#if rebuildFailure}<InlineNotice tone="danger">
        {rebuildFailure}
      </InlineNotice>{/if}
    {#if recordError}<InlineNotice tone="danger">
        {recordError}
      </InlineNotice>{/if}
    {#if capabilityError}<InlineNotice tone="warning">
        {m.s3_repo_mgmt_partially_unavailable({ error: capabilityError })}
      </InlineNotice>{/if}
    {#if repositoryError}<InlineNotice tone="danger">
        {hasRepositories
          ? m.s3_git_refresh_failed_keep_snapshot({ error: repositoryError })
          : m.s3_git_read_failed_sessions_ok({ error: repositoryError })}
      </InlineNotice>{/if}
  </div>
</ScrollArea>
