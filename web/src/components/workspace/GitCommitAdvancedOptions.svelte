<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
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
          : "Signed-off-by: (未配置提交者身份)"),
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
  <AdvancedSection title="高级提交选项" {summary} bind:open>
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
            修正上一次提交 (<span class="font-mono">--amend</span>，以
            <span class="font-mono">HEAD^ → index</span> 重写当前 HEAD)
            {#if unbornHead}
              <Badge variant="outline" class="ml-1">空仓无 HEAD 可修正</Badge>
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
            检测到当前已有未提交说明草稿，请选择如何与上一条 HEAD 提交说明合并：
          </p>
          <p class="line-clamp-2 font-mono text-muted-foreground">
            HEAD 说明：{draftPrompt.headMessage}
          </p>
          <div class="flex flex-wrap items-center gap-1.5">
            <Button
              size="xs"
              variant="default"
              onclick={() => onResolveAmendDraft("replace")}
            >
              替换为 HEAD 说明
            </Button>
            <Button
              size="xs"
              variant="outline"
              onclick={() => onResolveAmendDraft("merge")}
            >
              合并两份说明
            </Button>
            <Button
              size="xs"
              variant="ghost"
              onclick={() => onResolveAmendDraft("keep")}
            >
              保留当前草稿
            </Button>
          </div>
        </div>
      {/if}

      {#if amend && amendPushed}
        <InlineNotice tone="danger">
          <span data-testid="git-amend-pushed-warning">
            该提交已推送至远端，修正将改写提交历史并导致分支与上游分叉（不提供一键强推）。
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
              添加开发者签名尾注 (<span class="font-mono">-s / --signoff</span>)
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
            覆盖本次提交作者 (<span class="font-mono">--author</span>，选填)
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
