<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    conflict,
    saving = false,
    onKeepLocal,
    onAdoptRemote,
    onMergeBoth,
  }: {
    conflict: GitCommitDraftConflictState;
    saving?: boolean;
    onKeepLocal: () => void;
    onAdoptRemote: () => void;
    onMergeBoth: () => void;
  } = $props();
</script>

<div
  class="space-y-2.5 rounded-md border border-warning/40 bg-background p-2.5 text-xs"
  role="alert"
  data-testid="git-commit-draft-conflict"
>
  <InlineNotice tone="warning">
    {conflict.message || m.s6_draft_conflict_fallback_msg()}
  </InlineNotice>

  <div class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2">
    <div
      class="min-w-0 space-y-1.5 rounded-sm border bg-background p-2"
      data-testid="git-draft-conflict-local"
    >
      <div class="flex min-w-0 items-center justify-between gap-1.5">
        <span class="truncate font-semibold text-foreground">
          {m.s6_draft_conflict_local_heading()}
        </span>
        <Badge variant="outline">{m.s6_draft_conflict_unsynced()}</Badge>
      </div>
      <p
        class="truncate font-mono font-medium text-foreground"
        title={conflict.localSubject || m.s6_draft_conflict_empty_subject()}
      >
        {conflict.localSubject || m.s6_draft_conflict_empty_subject()}
      </p>
      {#if conflict.localBody.trim()}
        <div class="rounded-sm bg-background">
          <ScrollArea viewportClasses="max-h-24">
            <pre
              class="p-1.5 font-mono text-xs whitespace-pre-wrap break-words text-muted-foreground">{conflict.localBody}</pre>
          </ScrollArea>
        </div>
      {:else}
        <p class="text-muted-foreground">{m.s6_draft_conflict_empty_body()}</p>
      {/if}
    </div>

    <div
      class="min-w-0 space-y-1.5 rounded-sm border bg-background p-2"
      data-testid="git-draft-conflict-remote"
    >
      <div class="flex min-w-0 items-center justify-between gap-1.5">
        <span class="truncate font-semibold text-foreground">
          {m.s6_draft_conflict_remote_heading()}
        </span>
        <Badge variant="info">rev #{conflict.remoteDraft.revision}</Badge>
      </div>
      <p
        class="truncate font-mono font-medium text-foreground"
        title={conflict.remoteDraft.subject ||
          m.s6_draft_conflict_empty_subject()}
      >
        {conflict.remoteDraft.subject || m.s6_draft_conflict_empty_subject()}
      </p>
      {#if conflict.remoteDraft.body.trim()}
        <div class="rounded-sm bg-background">
          <ScrollArea viewportClasses="max-h-24">
            <pre
              class="p-1.5 font-mono text-xs whitespace-pre-wrap break-words text-muted-foreground">{conflict
                .remoteDraft.body}</pre>
          </ScrollArea>
        </div>
      {:else}
        <p class="text-muted-foreground">{m.s6_draft_conflict_empty_body()}</p>
      {/if}
    </div>
  </div>

  <div class="flex min-w-0 flex-wrap items-center justify-end gap-1.5">
    <Button
      size="sm"
      variant="outline"
      disabled={saving}
      onclick={onAdoptRemote}
      data-testid="git-draft-adopt-remote"
    >
      {m.s6_draft_conflict_adopt_remote()}
    </Button>
    <Button
      size="sm"
      variant="outline"
      disabled={saving}
      onclick={onKeepLocal}
      data-testid="git-draft-keep-local"
    >
      {m.s6_draft_conflict_keep_local()}
    </Button>
    <Button
      size="sm"
      variant="default"
      disabled={saving}
      onclick={onMergeBoth}
      data-testid="git-draft-merge-both"
    >
      {m.s6_draft_conflict_merge_both()}
    </Button>
  </div>
</div>
