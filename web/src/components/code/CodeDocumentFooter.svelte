<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import IconPlus from "~icons/tabler/plus";
  import CodeDownloadButton from "./CodeDownloadButton.svelte";
  import { basename, codeRawPath } from "../../domains/code/code-image";
  import { sourceLabel } from "../../domains/code/code-diff-client";
  import type { CodeDiff, CodeFile } from "../../contracts/schemas";

  let {
    id,
    snapshotId,
    rawSide,
    doc,
    shownDiff,
    side,
    hint,
    start,
    end,
    hasLocation,
    status,
    imageDocument,
    pdfFile,
    error,
    oninsert,
    ondownloaderror,
  }: {
    id: string;
    snapshotId: string;
    rawSide?: "old" | "new";
    doc: CodeFile["document"] | CodeDiff["old"] | undefined;
    shownDiff?: CodeDiff;
    side: "old" | "new";
    hint: string;
    start?: number;
    end?: number;
    hasLocation: boolean;
    status: string;
    imageDocument: boolean;
    pdfFile: boolean;
    error: string;
    oninsert: () => void;
    ondownloaderror: (message: string, code: number) => void;
  } = $props();
</script>

<div class="mt-auto shrink-0 space-y-1 border-t p-2">
  {#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
  <div class="flex flex-wrap items-center gap-2">
    <!-- Zero basis: the text truncates instead of wrapping the buttons. -->
    <p class="min-w-0 flex-1 basis-0 truncate text-xs text-muted-foreground">
      {#if shownDiff}{side === "old" ? m.s5_before() : m.s5_after()} · {sourceLabel(
          doc?.source ?? "",
        )}{/if}{#if hint}<span class="max-sm:hidden"
          >{shownDiff ? " · " : ""}<span>{hint}</span></span
        >{/if}
    </p>
    {#if doc && ((imageDocument && doc.source !== "empty") || (pdfFile && doc.kind !== "pdf"))}<CodeDownloadButton
        path={codeRawPath(id, snapshotId, rawSide)}
        filename={basename(doc.path)}
        disabled={!doc.downloadable}
        onerror={ondownloaderror}
      />{#if !doc.downloadable}<p
          class="self-center text-xs text-muted-foreground"
        >
          {m.s5_exceeds_download_limit()}
        </p>{/if}{/if}
    {#if start !== undefined && hasLocation}
      {@const label =
        end !== undefined && end !== start
          ? m.s5_insert_selected_lines({ range: `${start}-${end}` })
          : m.s5_insert_selected_line({ line: String(start) })}
      <Button
        size="icon-sm"
        aria-label={label}
        title={label}
        disabled={status !== "unchanged"}
        onclick={oninsert}
      >
        <IconPlus aria-hidden="true" />
      </Button>
    {/if}
  </div>
</div>
