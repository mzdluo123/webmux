<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitWorktreeContext } from "../../contracts/git-workbench-schemas";
  import { validateAuthorOverrideInput } from "../../domains/workspace/git-v2-adapter";
  import type { GitAmendDraftPrompt } from "../../hooks/workspace/git-commit-options.svelte";
  import AdvancedSection from "../shared/AdvancedSection.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    context,
    readonlyMode = false,
    amend = false,
    amendPushed = false,
    authorOverride = "",
    signoff = false,
    signoffTrailer = "",
    draftPrompt = null,
    onToggleAmend,
    onResolveAmendDraft,
    onAuthorOverrideChange,
    onToggleSignoff,
  }: {
    context: GitWorktreeContext;
    readonlyMode?: boolean;
    amend?: boolean;
    amendPushed?: boolean;
    authorOverride?: string;
    signoff?: boolean;
    signoffTrailer?: string;
    draftPrompt?: GitAmendDraftPrompt | null;
    onToggleAmend?: (checked: boolean) => void;
    onResolveAmendDraft?: (choice: "replace" | "merge" | "keep") => void;
    onAuthorOverrideChange?: (value: string) => void;
    onToggleSignoff?: (checked: boolean) => void;
  } = $props();

  let open = $state(false);

  const unbornHead = $derived(context.headState === "unborn");
  const authorValidation = $derived(
    validateAuthorOverrideInput(authorOverride),
  );
  const effectiveTrailer = $derived(
    signoffTrailer ||
      (context.committerName && context.committerEmail
        ? `Signed-off-by: ${context.committerName} <${context.committerEmail}>`
        : context.authorName && context.authorEmail
          ? `Signed-off-by: ${context.authorName} <${context.authorEmail}>`
          : m.s6_signoff_unconfigured_trailer()),
  );
  const summary = $derived.by(() => {
    const parts: string[] = [];
    if (amend) parts.push("Amend");
    if (signoff) parts.push("Sign-off");
    if (authorOverride.trim()) parts.push(`Author: ${authorOverride.trim()}`);
    return parts.join(" · ");
  });

  $effect(() => {
    if (amend || signoff || authorOverride.trim() || draftPrompt) {
      open = true;
    }
  });
</script>

<div
  class="rounded-md border bg-background px-1.5 py-0.5"
  data-testid="git-commit-advanced-options"
>
  <AdvancedSection title={m.s6_commit_advanced_title()} {summary} bind:open>
    <div class="space-y-2.5 pb-1 text-xs">
      {#if onToggleAmend}
        <Label class="items-start">
          <Checkbox
            checked={amend}
            disabled={readonlyMode || unbornHead}
            onCheckedChange={(val) => onToggleAmend(Boolean(val))}
            class="mt-0.5"
            data-testid="git-commit-amend-checkbox"
          />
          <span class="leading-snug">
            {m.s6_commit_amend_before()}<span class="font-mono">--amend</span
            >{m.s6_commit_amend_mid()}<span class="font-mono"
              >HEAD^ → index</span
            >{m.s6_commit_amend_after()}
            {#if unbornHead}
              <Badge variant="outline" class="ml-1">
                {m.s6_commit_amend_unborn_badge()}
              </Badge>
            {/if}
          </span>
        </Label>
      {/if}

      {#if draftPrompt && onResolveAmendDraft}
        <div
          class="space-y-2 rounded-md border bg-background p-2.5"
          data-testid="git-amend-draft-prompt"
        >
          <p class="font-medium text-foreground">
            {m.s6_amend_draft_prompt_title()}
          </p>
          <p class="line-clamp-2 font-mono text-muted-foreground">
            {m.s6_amend_draft_head_msg({ message: draftPrompt.headMessage })}
          </p>
          <div class="flex flex-wrap items-center gap-1.5">
            <Button
              size="sm"
              variant="default"
              onclick={() => onResolveAmendDraft("replace")}
            >
              {m.s6_amend_draft_replace()}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onclick={() => onResolveAmendDraft("merge")}
            >
              {m.s6_amend_draft_merge()}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onclick={() => onResolveAmendDraft("keep")}
            >
              {m.s6_amend_draft_keep()}
            </Button>
          </div>
        </div>
      {/if}

      {#if amend && amendPushed}
        <InlineNotice tone="danger">
          <span data-testid="git-amend-pushed-warning">
            {m.s6_amend_pushed_warning()}
          </span>
        </InlineNotice>
      {/if}

      {#if onToggleSignoff}
        <div class="space-y-1">
          <Label class="items-start">
            <Checkbox
              checked={signoff}
              disabled={readonlyMode}
              onCheckedChange={(val) => onToggleSignoff(Boolean(val))}
              class="mt-0.5"
              data-testid="git-commit-signoff-checkbox"
            />
            <span class="leading-snug">
              {m.s6_commit_signoff_before()}<span class="font-mono"
                >-s / --signoff</span
              >)
            </span>
          </Label>
          {#if signoff}
            <p
              class="pl-6 font-mono wrap-anywhere text-muted-foreground"
              data-testid="git-commit-signoff-preview"
            >
              {effectiveTrailer}
            </p>
          {/if}
        </div>
      {/if}

      {#if onAuthorOverrideChange}
        <div class="space-y-1">
          <Label for="git-commit-author-override">
            {m.s6_commit_author_override_before()}<span class="font-mono"
              >--author</span
            >{m.s6_commit_author_override_after()}
          </Label>
          <Input
            id="git-commit-author-override"
            size="sm"
            mono
            value={authorOverride}
            placeholder="Name <email@example.com>"
            disabled={readonlyMode}
            aria-invalid={!authorValidation.valid}
            oninput={(e) => onAuthorOverrideChange(e.currentTarget.value)}
            data-testid="git-commit-author-input"
          />
          {#if !authorValidation.valid}
            <p class="text-destructive" role="alert">
              {authorValidation.error}
            </p>
          {/if}
        </div>
      {/if}
    </div>
  </AdvancedSection>
</div>
