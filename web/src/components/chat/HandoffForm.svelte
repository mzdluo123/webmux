<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as Select from "$lib/components/ui/select";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import {
    emptyHandoffLocation,
    type HandoffEditor,
  } from "../../hooks/sessions/handoff.svelte";
  import type { TargetKind } from "../../domains/sessions/new-session-defaults";
  import { handoffMaxPages } from "../../contracts/handoff-schemas";
  import { getShell } from "../../hooks/shared/shell.svelte";
  import { getFeatureContributions } from "../../features/context";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";
  import HandoffPreviewCard from "./HandoffPreviewCard.svelte";
  let { editor: e, fork }: { editor: HandoffEditor; fork: boolean } = $props();
  const shell = getShell();
  const { LocationPicker, WorktreeCreateFields } =
    getFeatureContributions().creation;
  const workspaceAvailable = featureAvailable("workspace.manage");
  const agents = $derived(
    shell.launchConfigs?.state.status === "ready"
      ? shell.launchConfigs.state.configs.filter((item) => item.chat)
      : [],
  );
  const repositories = $derived(shell.repositories?.list ?? []);
  const agent = $derived(agents.find((item) => item.id === e.configId));
  const repository = $derived(
    repositories.find((item) => item.id === e.location.repositoryId),
  );
  let worktreeFields = $state<{ isReady: () => boolean }>();
  let managedPath = $state("");
  let formatFields = $state<HTMLDivElement | null>(null);
  let targetFields = $state<HTMLDivElement | null>(null);
  const imageFormat = $derived(e.format === "image");
  const existingTarget = $derived(e.targetSessionId !== "");
  useDockMotion(
    () => formatFields,
    () => imageFormat,
  );
  useDockMotion(
    () => targetFields,
    () => !existingTarget,
  );
  const locationReady = $derived(
    e.targetSessionId !== "" ||
      (e.locationKind === "directory"
        ? !!e.location.cwd.trim()
        : e.locationKind === "new-worktree"
          ? (worktreeFields?.isReady() ?? false)
          : !!e.location.repositoryId),
  );
  const asyncBusy = $derived(e.loading || e.busy);
  const generateInvalid = $derived(
    e.active || (!existingTarget && !agent) || !locationReady,
  );
  const generateDisabled = $derived(!asyncBusy && generateInvalid);
  const sendInvalid = $derived(
    e.active ||
      e.sealed ||
      !e.preview ||
      !!e.preview.blockedReason ||
      e.choiceChanged,
  );
  const sendDisabled = $derived(!asyncBusy && sendInvalid);
  function kind(next: TargetKind) {
    e.locationKind = next;
    const old = untrack(() => e.location);
    e.location =
      next === "directory"
        ? { ...emptyHandoffLocation(), cwd: old.cwd }
        : { ...emptyHandoffLocation(), repositoryId: old.repositoryId };
  }
  function repositoryId(value: string) {
    e.location = { ...emptyHandoffLocation(), repositoryId: value };
  }
  function worktreeId(value: string) {
    e.location = {
      ...untrack(() => e.location),
      worktreeId: value,
      cwd: "",
      newWorktreeBranch: "",
      newWorktreeBase: "",
    };
  }
</script>

<div class="min-w-0 space-y-4" data-handoff-form>
  {#if !e.source && e.error}<InlineNotice tone="danger">{e.error}</InlineNotice
    >{/if}
  <fieldset class="min-w-0" disabled={e.busy || e.active}>
    <div class="space-y-2 pb-3">
      <Label>{m.handoff_format()}</Label>
      <SegmentedControl
        value={e.format}
        onValueChange={(value) => {
          if (value === "image" || value === "text") e.format = value;
        }}
        aria-label={m.handoff_format()}
        size="form"
        wrap
      >
        <SegmentedControlItem value="image"
          >{m.handoff_format_image()}</SegmentedControlItem
        >
        <SegmentedControlItem value="text"
          >{m.handoff_format_text()}</SegmentedControlItem
        >
      </SegmentedControl>
    </div>
    <div bind:this={formatFields} hidden={!imageFormat} inert={!imageFormat}>
      <div class="space-y-2 pb-3">
        <Label for="handoff-pages"
          >{m.handoff_max_pages({ max: handoffMaxPages })}</Label
        >
        <Input
          id="handoff-pages"
          inputmode="numeric"
          autocomplete="off"
          bind:value={e.maxPagesDraft}
          onblur={() => e.commitMaxPages()}
          aria-invalid={!!e.maxPagesError}
          aria-describedby={e.maxPagesError
            ? "handoff-pages-help handoff-pages-error"
            : "handoff-pages-help"}
        />
        <p id="handoff-pages-help" class="text-xs text-muted-foreground">
          {m.handoff_image_reselect()}
        </p>
      </div>
      {#if e.maxPagesError}
        <p
          id="handoff-pages-error"
          class="pb-3 text-xs text-destructive"
          role="alert"
        >
          {e.maxPagesError}
        </p>
      {/if}
    </div>
    {#if !fork}<div class="space-y-2 pb-3">
        <Label for="handoff-target">{m.handoff_existing_target()}</Label><Input
          id="handoff-target"
          bind:value={e.targetSessionId}
        />
      </div>{/if}
    <div
      bind:this={targetFields}
      hidden={existingTarget}
      inert={existingTarget}
    >
      <div class="space-y-3 pb-3">
        <div class="space-y-2">
          <Label for="handoff-agent">{m.s3_handoff_target_agent()}</Label>
          <Select.Root type="single" bind:value={e.configId}>
            <Select.Trigger id="handoff-agent"
              >{agent?.name ??
                m.s3_handoff_select_target_agent()}</Select.Trigger
            >
            <Select.Content
              >{#each agents as item (item.id)}<Select.Item
                  value={item.id}
                  label={item.name}>{item.name}</Select.Item
                >{/each}</Select.Content
            >
          </Select.Root>
        </div>
        <div class="space-y-2">
          <Label for="handoff-model">{m.s3_handoff_model_optional()}</Label
          ><Input id="handoff-model" bind:value={e.model} />
        </div>
        {#if workspaceAvailable && LocationPicker}
          <LocationPicker
            {repositories}
            recent={[]}
            preferredRepositoryId={e.source?.workspace?.repositoryId ?? ""}
            createdTree={null}
            newWorktreeSummary={e.location.newWorktreeBranch ||
              m.s3_handoff_create_worktree_on_send()}
            bind:kind={() => e.locationKind, kind}
            bind:cwd={
              () => e.location.cwd,
              (value) =>
                (e.location = { ...untrack(() => e.location), cwd: value })
            }
            bind:repositoryId={() => e.location.repositoryId, repositoryId}
            bind:worktreeId={() => e.location.worktreeId, worktreeId}
            disabled={e.busy || e.active}
          />
        {:else}
          <div class="min-w-0 space-y-2">
            <Label for="handoff-cwd">{m.s3_working_directory()}</Label>
            <Input
              id="handoff-cwd"
              value={e.location.cwd}
              oninput={(event) => {
                e.location = {
                  ...emptyHandoffLocation(),
                  cwd: event.currentTarget.value,
                };
                e.locationKind = "directory";
              }}
              placeholder={m.s3_enter_abs_dir_placeholder()}
              autocomplete="off"
              disabled={e.busy || e.active}
            />
          </div>
        {/if}
        {#if workspaceAvailable && WorktreeCreateFields && e.locationKind === "new-worktree" && repository}
          {#key repository.id}<WorktreeCreateFields
              bind:this={worktreeFields}
              {repository}
              deferred
              preset={{
                mode: "new",
                name: e.location.newWorktreeBranch,
                base: e.location.newWorktreeBase,
              }}
              mode="new"
              bind:path={managedPath}
              bind:branch={
                () => e.location.newWorktreeBranch,
                (value) =>
                  (e.location = {
                    ...untrack(() => e.location),
                    newWorktreeBranch: value,
                  })
              }
              bind:base={
                () => e.location.newWorktreeBase,
                (value) =>
                  (e.location = {
                    ...untrack(() => e.location),
                    newWorktreeBase: value,
                  })
              }
              disabled={e.busy || e.active}
            />{/key}
          <p class="text-xs text-muted-foreground">
            {m.s3_handoff_worktree_deferred_hint()}
          </p>
        {/if}
      </div>
    </div>
    <div class="space-y-2">
      <Label for="handoff-task">{m.s3_user_instruction()}</Label><Textarea
        id="handoff-task"
        rows={4}
        bind:value={e.task}
      />
    </div>
  </fieldset>
  <div class="flex flex-wrap gap-2">
    <Button
      variant="outline"
      disabled={generateDisabled}
      aria-disabled={asyncBusy || undefined}
      tabindex={asyncBusy ? -1 : undefined}
      onclick={() => {
        if (asyncBusy || generateInvalid) return;
        void e.generate();
      }}>{m.s3_handoff_generate_preview()}</Button
    >
    <Button
      disabled={sendDisabled}
      aria-disabled={asyncBusy || undefined}
      tabindex={asyncBusy ? -1 : undefined}
      onclick={() => {
        if (asyncBusy || sendInvalid) return;
        void e.send();
      }}>{m.s3_handoff_send()}</Button
    >
  </div>
  <HandoffPreviewCard editor={e} />
</div>
