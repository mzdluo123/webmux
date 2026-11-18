<script lang="ts">
  import { untrack } from "svelte";
  import GitCommitComposer from "../../src/components/workspace/GitCommitComposer.svelte";
  import type { GitWorktreeContext } from "../../src/contracts/git-workbench-schemas";
  import {
    joinCommitMessageDraft,
    mergeCommitMessageDrafts,
    type GitCommitDraftConflictState,
  } from "../../src/domains/workspace/git-commit-draft-model";

  let {
    context,
    initialMessage,
    initialConflict,
  }: {
    context: GitWorktreeContext;
    initialMessage: string;
    initialConflict: GitCommitDraftConflictState;
  } = $props();

  let commitMessage = $state(untrack(() => initialMessage));
  let draftConflict = $state<GitCommitDraftConflictState | null>(
    untrack(() => initialConflict),
  );
</script>

<div class="git-workbench bg-background">
  <GitCommitComposer
    {context}
    stagedCount={1}
    {commitMessage}
    {draftConflict}
    onCommitMessageChange={(next) => {
      commitMessage = next;
    }}
    onOpenCommitPreview={() => {}}
    onResolveDraftKeepLocal={() => {
      draftConflict = null;
    }}
    onResolveDraftAdoptRemote={() => {
      if (!draftConflict) return;
      commitMessage = joinCommitMessageDraft(
        draftConflict.remoteDraft.subject,
        draftConflict.remoteDraft.body,
      );
      draftConflict = null;
    }}
    onResolveDraftMergeBoth={() => {
      if (!draftConflict) return;
      const merged = mergeCommitMessageDrafts(
        {
          subject: draftConflict.localSubject,
          body: draftConflict.localBody,
        },
        {
          subject: draftConflict.remoteDraft.subject,
          body: draftConflict.remoteDraft.body,
        },
      );
      commitMessage = joinCommitMessageDraft(merged.subject, merged.body);
      draftConflict = null;
    }}
  />
</div>
