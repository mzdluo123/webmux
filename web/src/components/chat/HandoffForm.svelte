<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Select from "$lib/components/ui/select";
  import { m } from "$lib/paraglide/messages.js";
  import { uploadQuickStartFile } from "../../platform/api";
  import { emptyLocation } from "../../domains/sessions/handoff-draft";
  import type { HandoffEditor } from "../../hooks/sessions/handoff.svelte";
  import { useChatUpload } from "../../hooks/shared/upload.svelte";
  import type { TargetKind } from "../../domains/sessions/new-session-defaults";
  import { getShell } from "../../hooks/shared/shell.svelte";
  import ComposerCard from "./ComposerCard.svelte";
  import ChatUploadButton from "./ChatUploadButton.svelte";
  import HandoffPreviewCard from "./HandoffPreviewCard.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { getFeatureContributions } from "../../features/context";
  import { featureAvailable } from "../../platform/feature-host.svelte";

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
  const uploads = useChatUpload(
    uploadQuickStartFile,
    (message) => e.reportError(message),
    { get: () => e.attachments, set: (next) => (e.attachments = next) },
  );
  const unsupportedLocation = $derived(
    !workspaceAvailable &&
      Boolean(
        e.location.repositoryId ||
        e.location.worktreeId ||
        e.location.newWorktreeBranch ||
        e.location.newWorktreeBase,
      ),
  );
  const locationReady = $derived(
    unsupportedLocation
      ? false
      : !fork || e.locationKind === "directory"
        ? !fork || !!e.location.cwd.trim()
        : e.locationKind === "new-worktree"
          ? (worktreeFields?.isReady() ?? false)
          : !!e.location.repositoryId,
  );
  const regenerate = $derived(
    !e.preview ||
      e.sealed ||
      e.choiceChanged ||
      ((e.preview.stale || e.preview.budget.overBudget) &&
        e.delivery?.status !== "failed"),
  );
  const disabled = $derived(
    unsupportedLocation ||
      (e.delivery?.status === "pending"
        ? e.busy || e.saving || e.loading || !e.delivery.clientRequestId
        : e.busy ||
          e.saving ||
          !!uploads.progress ||
          e.loading ||
          !e.source ||
          e.conflict ||
          e.locked ||
          !agent ||
          (!locationReady &&
            !(e.delivery?.status === "failed" && !e.choiceChanged)) ||
          (!regenerate && !e.task.trim() && !e.attachments.length)),
  );
  const actionLabel = $derived(
    e.delivery?.status === "unknown"
      ? m.s3_handoff_wait_reconcile()
      : e.delivery?.status === "pending"
        ? m.s3_handoff_continue_prep()
        : regenerate
          ? e.preview
            ? m.s3_handoff_repreview()
            : m.s3_handoff_generate_preview()
          : e.delivery?.status === "failed"
            ? m.s3_handoff_retry()
            : m.s3_handoff_send(),
  );
  $effect(() => {
    if (e.sealed && e.preview) uploads.commit(e.preview.attachments);
  });
  function kind(next: TargetKind) {
    e.locationKind = next;
    const prior = untrack(() => e.location);
    e.location =
      next === "directory"
        ? { ...emptyLocation(), cwd: prior.cwd }
        : {
            ...emptyLocation(),
            repositoryId: prior.repositoryId,
            worktreeId: next === "worktree" ? prior.worktreeId : "",
          };
  }
  function repositoryId(value: string) {
    if (untrack(() => e.locationKind) === "directory") return;
    if (value !== untrack(() => e.location.repositoryId))
      e.location = { ...emptyLocation(), repositoryId: value };
  }
  function worktreeId(value: string) {
    if (untrack(() => e.locationKind) !== "worktree") return;
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
  {#if e.error}<InlineNotice tone="danger">{e.error}</InlineNotice>{/if}
  {#if e.conflict}
    <div class="flex flex-wrap gap-2">
      <Button
        variant="outline"
        disabled={e.busy}
        onclick={() => void e.serverVersion()}
        >{m.s3_handoff_load_server_version()}</Button
      >
      <Button
        variant="outline"
        disabled={e.busy ||
          !!uploads.progress ||
          e.locked ||
          !e.configId ||
          unsupportedLocation}
        onclick={() => void e.generate()}
        >{m.s3_handoff_keep_edits_repreview()}</Button
      >
    </div>
  {/if}
  <fieldset class="min-w-0 space-y-3" disabled={e.busy || e.locked}>
    <div class="grid min-w-0 gap-3 sm:grid-cols-2">
      <div class="min-w-0 space-y-2">
        <Label for="handoff-agent">{m.s3_handoff_target_agent()}</Label>
        <Select.Root
          type="single"
          value={e.configId}
          onValueChange={(value) => {
            e.configId = value;
            e.model = "";
          }}
        >
          <Select.Trigger
            id="handoff-agent"
            aria-label={m.s3_handoff_target_agent()}
            >{agent?.name ?? m.s3_handoff_select_target_agent()}</Select.Trigger
          >
          <Select.Content
            >{#each agents as item (item.id)}<Select.Item
                value={item.id}
                label={item.name}>{item.name}</Select.Item
              >{/each}</Select.Content
          >
        </Select.Root>
      </div>
      <div class="min-w-0 space-y-2">
        <Label for="handoff-model">{m.s3_handoff_model_optional()}</Label>
        <Input
          id="handoff-model"
          value={e.model}
          oninput={(event) => (e.model = event.currentTarget.value)}
          placeholder={m.s3_handoff_model_placeholder()}
          autocomplete="off"
        />
      </div>
    </div>
    {#if !agents.length}<p class="text-sm text-muted-foreground">
        {m.s3_handoff_no_chat_configs()}
      </p>{/if}
    {#if fork}
      <p class="break-all text-xs text-muted-foreground">
        {m.s3_handoff_source_location_hint({
          cwd: e.preview
            ? e.preview.sourceLocation.cwd ||
              m.s3_handoff_source_cwd_unrecorded()
            : (e.source?.cwd ?? ""),
        })}
      </p>
      {#if workspaceAvailable && LocationPicker}
        <LocationPicker
          {repositories}
          recent={[]}
          preferredRepositoryId={e.source?.repositoryId ?? ""}
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
          disabled={e.busy || e.locked}
        />
      {:else}
        <div class="min-w-0 space-y-2">
          <Label for="handoff-cwd">{m.s3_working_directory()}</Label>
          <Input
            id="handoff-cwd"
            value={e.location.cwd}
            oninput={(event) => {
              e.location = {
                ...emptyLocation(),
                cwd: event.currentTarget.value,
              };
              e.locationKind = "directory";
            }}
            placeholder={m.s3_enter_abs_dir_placeholder()}
            autocomplete="off"
            disabled={e.busy || e.locked}
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
            disabled={e.busy || e.locked}
          />{/key}
        <p class="text-xs text-muted-foreground">
          {m.s3_handoff_worktree_deferred_hint()}
        </p>
      {/if}
    {:else}
      <div class="min-w-0 space-y-1 text-sm">
        <Label>{m.s3_handoff_location_unchanged()}</Label>
        <p class="break-all text-muted-foreground">{e.source?.cwd ?? ""}</p>
      </div>
    {/if}
    {#if unsupportedLocation}
      <InlineNotice tone="warning">
        {m.s3_handoff_workspace_unavailable()}
        <Button
          variant="outline"
          size="sm"
          disabled={e.busy || e.locked}
          onclick={() => {
            e.location = { ...emptyLocation(), cwd: e.location.cwd };
            e.locationKind = "directory";
          }}>{m.s3_handoff_use_existing_dir()}</Button
        >
      </InlineNotice>
    {/if}
  </fieldset>
  {#if e.choiceChanged && !e.sealed}<InlineNotice tone="warning"
      >{m.s3_handoff_choice_changed_notice()}</InlineNotice
    >{/if}
  <HandoffPreviewCard editor={e} />
  <ComposerCard
    value={e.task}
    oninput={(event) => (e.task = event.currentTarget.value)}
    label={m.s3_handoff_task_label()}
    placeholder={m.s3_handoff_task_placeholder()}
    {uploads}
    disabled={e.busy || e.locked}
    uploadsDisabled={e.busy || e.locked}
    busy={e.busy || !!uploads.progress || !!e.preview}
    action={{
      kind: "send",
      label: actionLabel,
      disabled,
      onclick: () =>
        void (e.delivery?.status === "pending"
          ? e.send()
          : regenerate
            ? e.generate()
            : e.send()),
    }}
  >
    {#snippet leading()}<ChatUploadButton
        {uploads}
        disabled={e.busy || e.locked}
      />{/snippet}
    {#snippet hints()}<p class="px-1 text-xs text-muted-foreground">
        {e.locked
          ? m.s3_handoff_hint_locked()
          : e.saving
            ? m.s3_handoff_hint_saving_task()
            : e.preview && !e.dirty && !e.sealed
              ? m.s3_handoff_hint_task_saved()
              : m.s3_handoff_hint_separate_save()}
      </p>{/snippet}
  </ComposerCard>
  {#if e.preview && !e.locked}
    <div class="flex flex-wrap gap-2">
      <Button
        variant="outline"
        disabled={e.busy ||
          !!uploads.progress ||
          !e.source ||
          !agent ||
          !locationReady}
        onclick={() => void e.generate()}
        >{e.preview.manual || e.context !== e.preview.renderedText
          ? m.s3_handoff_repreview_keep_edits()
          : m.s3_handoff_repreview()}</Button
      >
    </div>
  {/if}
</div>
