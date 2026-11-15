<script lang="ts">
  import IconCloudDownload from "~icons/tabler/cloud-download";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Button } from "$lib/components/ui/button";
  import type { FetchResult } from "../../contracts/schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitWorkbenchButton from "./GitWorkbenchButton.svelte";

  let {
    mode = "actions",
    fetching = false,
    fetchBlocked = "",
    fetchError = "",
    fetchResults = null,
    failedFetches = 0,
    checkedAt = "",
    refreshing = false,
    refreshError = "",
    onfetch,
    ongit,
  }: {
    mode?: "actions" | "notices";
    fetching?: boolean;
    fetchBlocked?: string;
    fetchError?: string;
    fetchResults?: FetchResult[] | null;
    failedFetches?: number;
    checkedAt?: string;
    refreshing?: boolean;
    refreshError?: string;
    onfetch?: () => void;
    ongit?: () => void;
  } = $props();

  const fetchReasonId = $props.id();
  const fetchLabel = $derived(
    fetching ? m.s8_fetching_remotes_aria() : m.s8_fetch_remotes(),
  );
</script>

{#if mode === "actions"}
  <div class="inline-flex items-center gap-1.5">
    {#if ongit}<GitWorkbenchButton {ongit} />{/if}
    <!-- A disabled button shows no hint, so the wrapper carries the reason. -->
    <span class="inline-flex" title={fetchBlocked || fetchLabel}>
      <Button
        variant="outline"
        size="sm"
        class="max-sm:size-(--control-default)"
        aria-label={fetchLabel}
        aria-describedby={fetchBlocked ? fetchReasonId : undefined}
        aria-busy={fetching}
        disabled={fetching || !!fetchBlocked}
        onclick={onfetch}
      >
        {#if fetching}
          <IconLoader2
            aria-hidden="true"
            class="animate-spin motion-reduce:animate-none"
          />
        {:else}
          <IconCloudDownload aria-hidden="true" />
        {/if}
        <span class="hidden sm:inline">
          {fetching ? m.s8_fetching_short() : m.s8_fetch_remotes()}
        </span>
      </Button>
      {#if fetchBlocked}
        <span id={fetchReasonId} class="sr-only">{fetchBlocked}</span>
      {/if}
    </span>
  </div>
{:else}
  {#if refreshing}
    <p role="status" class="text-xs text-muted-foreground">
      {m.s8_rereading_status_notice()}
    </p>
  {:else if refreshError}
    <InlineNotice tone="danger">
      {m.s8_reread_failed_notice({
        error: refreshError,
        time: new Date(checkedAt).toLocaleString(locale.current),
      })}
    </InlineNotice>
  {/if}
  {#if fetchError}
    <InlineNotice tone="danger"
      >{m.s8_fetch_remotes_failed({ error: fetchError })}</InlineNotice
    >
  {:else if fetchResults}
    {@const tone =
      failedFetches === 0
        ? "success"
        : failedFetches === fetchResults.length
          ? "danger"
          : "warning"}
    <InlineNotice {tone}>
      <ul class="space-y-1" aria-label={m.s8_fetch_results_aria()}>
        {#each fetchResults as result (result.remote)}
          <li class="wrap-anywhere">
            <span class="font-medium">{result.remote}</span
            >{m.s8_colon_sep()}{result.status === "ok"
              ? m.s8_fetch_status_ok()
              : result.message || m.s8_fetch_status_failed()}
          </li>
        {/each}
      </ul>
    </InlineNotice>
  {/if}
{/if}
