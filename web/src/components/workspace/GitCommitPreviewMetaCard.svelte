<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitCommitPreviewModel } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    model,
    acknowledgePushedAmend = false,
    submitting = false,
    onAcknowledgePushedAmendChange,
  }: {
    model: GitCommitPreviewModel;
    acknowledgePushedAmend?: boolean;
    submitting?: boolean;
    onAcknowledgePushedAmendChange?: (checked: boolean) => void;
  } = $props();

  const distinctCommitter = $derived(
    Boolean(
      model.committerName &&
      (model.committerName !== model.authorName ||
        model.committerEmail !== model.authorEmail),
    ),
  );
</script>

<div class="space-y-2.5">
  <div
    class="grid min-w-0 grid-cols-1 gap-1.5 rounded-lg border bg-background p-3 text-xs sm:grid-cols-2"
  >
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">{m.s6_meta_target_branch()}</span>
      <span class="font-medium text-foreground">{model.branch}</span>
      {#if model.amend}
        <Badge variant="warning" class="ml-1">{m.s6_meta_amend_badge()}</Badge>
      {/if}
    </p>
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">{m.s6_meta_worktree()}</span>
      <span class="font-mono text-foreground">{model.worktreePath}</span>
    </p>
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">
        {model.authorOverride
          ? m.s6_meta_author_override()
          : m.s6_meta_author()}
      </span>
      <span class="text-foreground">
        {model.authorName || m.s6_meta_unconfigured_parens()} &lt;{model.authorEmail ||
          m.s6_meta_unconfigured()}&gt;
      </span>
    </p>
    {#if distinctCommitter || model.authorOverride}
      <p class="wrap-anywhere">
        <span class="text-muted-foreground">{m.s6_meta_committer()}</span>
        <span class="text-foreground">
          {model.committerName || m.s6_meta_unconfigured_parens()} &lt;{model.committerEmail ||
            m.s6_meta_unconfigured()}&gt;
        </span>
      </p>
    {/if}
    <p class="wrap-anywhere">
      <span class="text-muted-foreground">{m.s6_meta_signing_and_hooks()}</span>
      <span class="text-foreground">
        {model.signEnabled
          ? model.signFormat
            ? m.s6_meta_sign_format_enabled({
                format: model.signFormat.toUpperCase(),
              })
            : m.s6_meta_sign_enabled()
          : m.s6_meta_sign_disabled()}
        {#if model.signoff}
          {m.s6_meta_with_signoff()}
        {/if}
        ·
        {model.executableHooks.length > 0
          ? model.executableHooks.join(", ")
          : m.s6_meta_no_hooks()}
      </span>
    </p>
  </div>

  {#if model.amend && model.amendPushed}
    <div
      class="space-y-2 rounded-lg border border-destructive/50 bg-destructive/5 p-3 text-xs"
      data-testid="git-preview-pushed-amend-guard"
    >
      <InlineNotice tone="danger">
        {m.s6_meta_pushed_amend_notice()}
      </InlineNotice>
      <Label class="items-start">
        <Checkbox
          checked={acknowledgePushedAmend}
          disabled={submitting}
          onCheckedChange={(val) =>
            onAcknowledgePushedAmendChange?.(Boolean(val))}
          class="mt-0.5"
          data-testid="git-preview-ack-pushed-amend-checkbox"
        />
        <span class="leading-snug text-foreground">
          {m.s6_meta_pushed_amend_ack()}
        </span>
      </Label>
    </div>
  {/if}
</div>
