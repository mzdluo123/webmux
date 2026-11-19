<script lang="ts">
  import { tick } from "svelte";
  import type {
    AgentConnector,
    Repository,
    Project,
    SessionView,
  } from "../../contracts/schemas";
  import {
    visibleLaunchConfigs,
    type LaunchConfigsState,
  } from "../../domains/settings/launch-configs";
  import {
    formatSubmitButtonLabel,
    buildNewSessionSummary,
    formatLocationSummaryText,
    resolveEntryPreset,
    resolvePreferredStartBranch,
    type LaunchEntry,
    type LaunchSource,
  } from "../../domains/sessions/new-session-defaults";
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";
  import {
    featureAvailable,
    featureEnabled,
  } from "../../platform/feature-host.svelte";
  import type {
    CreationContributions,
    CreationLocationProps,
  } from "../../features/context";
  import IconFolder from "~icons/tabler/folder";
  import { m } from "$lib/paraglide/messages.js";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import SplitDirectionChoice from "../terminal/SplitDirectionChoice.svelte";
  import { AdvancedSection } from "$lib/components/ui/advanced-section";
  import DirectoryInput from "../shared/DirectoryInput.svelte";
  import FirstMessageFields from "./FirstMessageFields.svelte";
  import CustomCommandField from "./CustomCommandField.svelte";

  interface Props extends Omit<CreationLocationProps, "newWorktreeSummary"> {
    providers: CreationContributions;
    project?: Project;
    onprojectchange?: () => void;
    busy: boolean;
    fieldsRepository: Repository | undefined;
    worktreeFields: { isReady: () => boolean } | undefined;
    entry: LaunchEntry | undefined;
    newPath: string;
    branchMode: "new" | "existing";
    branch: string;
    base: string;
    startBranch: string;
    workspaceEnabled: boolean;
    intent: "session" | "worktree";
    withSession: boolean;
    treeOnly: boolean;
    launchConfigs: LaunchConfigsState;
    source: LaunchSource;
    configId: string;
    command: string;
    startSide: SessionView;
    onlaunchchoose: () => void;
    lastLaunch: string;
    instructionRepository: Repository | undefined;
    hasInstruction: boolean;
    useInstruction: boolean;
    prompt: string;
    startsOnChat: boolean;
    split: boolean;
    direction: SplitDirection;
    name: string;
    error: string;
    missingConnector: AgentConnector | undefined;
    targetReady: boolean;
    commandReady: boolean;
    oncreate: () => void;
    onterminalcreate: () => void;
    onclose: () => void;
  }

  let {
    providers,
    project,
    onprojectchange,
    repositories,
    scope,
    recent,
    preferredRepositoryId,
    createdTree,
    onremove,
    kind = $bindable(),
    cwd = $bindable(),
    repositoryId = $bindable(),
    worktreeId = $bindable(),
    busy,
    fieldsRepository,
    worktreeFields = $bindable(),
    entry,
    newPath = $bindable(),
    branchMode = $bindable(),
    branch = $bindable(),
    base = $bindable(),
    startBranch = $bindable(),
    workspaceEnabled,
    intent,
    withSession = $bindable(),
    treeOnly,
    launchConfigs,
    source = $bindable(),
    configId = $bindable(),
    command = $bindable(),
    startSide = $bindable(),
    onlaunchchoose,
    lastLaunch,
    instructionRepository,
    hasInstruction,
    useInstruction = $bindable(),
    prompt = $bindable(),
    startsOnChat,
    split,
    direction = $bindable(),
    name = $bindable(),
    error,
    missingConnector,
    targetReady,
    commandReady,
    oncreate,
    onterminalcreate,
    onclose,
  }: Props = $props();
  let worktreeFieldset = $state<HTMLFieldSetElement>();

  async function selectLocation(next: CreationLocationProps["kind"]) {
    const changed = kind !== next;
    kind = next;
    if (!changed || next !== "new-worktree") return;
    await tick();
    // The inline picker releases its list after the new fields mount.
    await new Promise<void>((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
    );
    if (kind !== next) return;
    const branchField = worktreeFieldset?.querySelector(
      "#worktree-branch, #existing-branch",
    );
    branchField?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  const repository = $derived(
    repositories.find((item) => item.id === repositoryId),
  );
  const worktree = $derived(
    createdTree?.id === worktreeId
      ? createdTree
      : repository?.worktrees.find((item) => item.id === worktreeId),
  );
  const chosenConfig = $derived(
    source === "config"
      ? visibleLaunchConfigs(launchConfigs).find((item) => item.id === configId)
      : undefined,
  );
  const treeTargetMissing = $derived(treeOnly && kind !== "new-worktree");
  const newWorktreeSummary = $derived(
    branchMode === "new"
      ? `${branch || m.s3_new_branch_fallback()} ← ${startBranch || "…"}`
      : m.s3_existing_branch_summary({
          branch: branch || m.s3_unselected_fallback(),
        }),
  );
  const locationText = $derived(
    formatLocationSummaryText(kind, cwd, repository, worktree, branch),
  );
  const summary = $derived(
    buildNewSessionSummary({
      treeTargetMissing,
      targetReady,
      treeOnly,
      kind,
      locationText,
      commandReady,
      source,
      chosenConfig,
      command,
    }),
  );
</script>

<form
  class="min-w-0"
  onsubmit={(event) => (event.preventDefault(), oncreate())}
>
  <fieldset disabled={busy} class="min-w-0 space-y-4">
    {#if project}
      <div class="space-y-2">
        <Label>{m.s3_project_label()}</Label>
        {#if onprojectchange}<Button
            variant="outline"
            wrap
            class="w-full justify-start"
            aria-label={m.s3_change_project_label()}
            onclick={onprojectchange}
            ><IconFolder aria-hidden="true" /><ChoiceDetails
              primary={project.name}
              secondary={project.path}
            /></Button
          >
        {:else}<div class="flex min-w-0 items-center gap-2">
            <IconFolder aria-hidden="true" /><ChoiceDetails
              primary={project.name}
              secondary={project.path}
            />
          </div>{/if}
        {#if project.error}<p
            role="status"
            class="break-words text-sm text-destructive"
          >
            {project.error}
          </p>{/if}
      </div>
    {/if}
    {#if workspaceEnabled && providers.LocationPicker && (!project || project.repositoryId)}
      <providers.LocationPicker
        {repositories}
        {scope}
        {recent}
        {preferredRepositoryId}
        {createdTree}
        {newWorktreeSummary}
        {onremove}
        bind:kind={() => kind, selectLocation}
        bind:cwd
        bind:repositoryId
        bind:worktreeId
        disabled={busy}
        projectScoped={!!project}
      />
    {:else if project?.repositoryId}
      <p role="status" class="text-sm text-muted-foreground">
        {m.s3_project_git_location_unavailable()}
      </p>
    {:else if !project}
      <div class="space-y-2">
        <Label for="cwd">{m.s3_working_directory()}</Label>
        <DirectoryInput
          bind:value={cwd}
          Picker={featureAvailable("files.upload")
            ? providers.DirectoryPicker
            : undefined}
        />
      </div>
    {/if}
    {#if featureEnabled("files") && (!featureAvailable("files.upload") || !providers.DirectoryPicker)}
      <p role="status" class="text-sm text-muted-foreground">
        {m.s3_files_temporarily_unavailable_hint()}
      </p>
    {/if}
    {#if featureEnabled("chat") && (!featureAvailable("chat.acp") || !providers.LaunchStartSideFields)}
      <p role="status" class="text-sm text-muted-foreground">
        {m.s3_chat_temporarily_unavailable_hint()}
      </p>
    {/if}
    {#if workspaceEnabled && fieldsRepository && providers.WorktreeCreateFields}
      <fieldset
        bind:this={worktreeFieldset}
        disabled={busy || kind !== "new-worktree"}
        hidden={kind !== "new-worktree"}
        class="min-w-0"
      >
        {#key fieldsRepository.id}<providers.WorktreeCreateFields
            bind:this={worktreeFields}
            repository={fieldsRepository}
            preferredStartBranch={resolvePreferredStartBranch(
              fieldsRepository,
              scope?.worktreeId,
            )}
            preset={resolveEntryPreset(entry, fieldsRepository.id)}
            disabled={busy || kind !== "new-worktree"}
            bind:path={newPath}
            bind:mode={branchMode}
            bind:branch
            bind:base
            bind:startBranch
          />{/key}
      </fieldset>
    {/if}
    {#if createdTree && kind === "worktree" && worktreeId === createdTree.id}
      <p role="status" class="text-sm text-primary">
        {m.s3_worktree_already_created_hint()}
      </p>
    {/if}
    {#if workspaceEnabled && intent === "worktree"}
      <div class="flex items-center gap-2">
        <Checkbox id="worktree-with-session" bind:checked={withSession} />
        <Label for="worktree-with-session"
          >{m.s3_start_session_after_create()}</Label
        >
      </div>
    {/if}
    {#if !treeOnly}
      {#if providers.LaunchPicker}
        <providers.LaunchPicker
          {launchConfigs}
          bind:source
          bind:configId
          bind:command
          bind:startSide
          disabled={busy}
          onchoose={onlaunchchoose}
        />
      {:else}
        <div class="space-y-2">
          <CustomCommandField
            bind:command
            showLabel
            disabled={busy}
            oninput={() => {
              source = "custom";
              onlaunchchoose();
            }}
          />
          {#if lastLaunch.startsWith("config:") && source === "unset"}
            <p role="status" class="text-sm text-muted-foreground">
              {featureEnabled("launch")
                ? m.s3_launch_ui_unavailable_reselect()
                : m.s3_launch_feature_disabled_reselect()}
            </p>
            <Button
              type="button"
              variant="outline"
              onclick={() => {
                source = "custom";
                onlaunchchoose();
              }}
            >
              {m.s3_use_custom_command()}
            </Button>
          {/if}
        </div>
      {/if}
      <FirstMessageFields
        repositoryName={instructionRepository?.name}
        {hasInstruction}
        bind:useInstruction
        bind:prompt
        chat={startsOnChat}
        disabled={busy}
      />
      {#if split}<SplitDirectionChoice bind:direction />{/if}
      <AdvancedSection
        title={m.s3_advanced_session_name_title()}
        summary={name}
      >
        <div class="space-y-2">
          <Label for="session-name"
            >{m.s3_session_name_label()}
            <span class="text-muted-foreground">{m.s3_optional_tag()}</span
            ></Label
          ><Input
            id="session-name"
            bind:value={name}
            placeholder={m.s3_session_name_placeholder()}
          />
        </div>
      </AdvancedSection>
    {/if}
    {#if error}<p role="alert" class="text-sm text-destructive">
        {error}
      </p>{/if}
    {#if missingConnector && providers.ConnectorInstallHint}
      <providers.ConnectorInstallHint connector={missingConnector}>
        {#snippet actions()}<Button
            type="button"
            size="sm"
            variant="outline"
            disabled={busy}
            onclick={onterminalcreate}
            >{m.s3_start_in_terminal_instead()}</Button
          >{/snippet}
      </providers.ConnectorInstallHint>
    {/if}
    <div class="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center">
      <p
        id="session-summary"
        class="min-w-0 flex-1 text-sm wrap-anywhere text-muted-foreground"
      >
        {summary}
      </p>
      <div class="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onclick={onclose}>{m.s3_cancel()}</Button
        >
        <Button
          type="submit"
          aria-describedby="session-summary"
          disabled={busy || !targetReady || (!treeOnly && !commandReady)}
          >{formatSubmitButtonLabel(busy, treeOnly, split)}</Button
        >
      </div>
    </div>
  </fieldset>
</form>
