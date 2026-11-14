<script lang="ts">
  import type {
    AgentConnector,
    Repository,
    Project,
    SessionView,
  } from "../../contracts/schemas";
  import type { LaunchConfigsState } from "../../domains/settings/launch-configs";
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
    pluginAvailable,
    pluginEnabled,
  } from "../../platform/plugin-host.svelte";
  import type {
    CreationContributions,
    CreationLocationProps,
  } from "../../plugins/context";
  import IconFolder from "~icons/tabler/folder";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import SplitDirectionChoice from "../terminal/SplitDirectionChoice.svelte";
  import AdvancedSection from "../shared/AdvancedSection.svelte";
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
  const repository = $derived(
    repositories.find((item) => item.id === repositoryId),
  );
  const worktree = $derived(
    createdTree?.id === worktreeId
      ? createdTree
      : repository?.worktrees.find((item) => item.id === worktreeId),
  );
  const chosenConfig = $derived(
    source === "config" && launchConfigs.status === "ready"
      ? launchConfigs.configs.find((item) => item.id === configId)
      : undefined,
  );
  const treeTargetMissing = $derived(treeOnly && kind !== "new-worktree");
  const newWorktreeSummary = $derived(
    branchMode === "new"
      ? `${branch || "新分支"} ← ${startBranch || "…"}`
      : `已有分支 ${branch || "未选择"}`,
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
        <Label>项目</Label>
        {#if onprojectchange}<Button
            variant="outline"
            wrap
            class="w-full justify-start"
            aria-label="更换项目"
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
        bind:kind
        bind:cwd
        bind:repositoryId
        bind:worktreeId
        disabled={busy}
        projectScoped={!!project}
      />
    {:else if project?.repositoryId}
      <p role="status" class="text-sm text-muted-foreground">
        当前项目的 Git 工作位置暂不可用，请检查服务端 Git 与仓库状态。
      </p>
    {:else if !project}
      <div class="space-y-2">
        <Label for="cwd">工作目录</Label>
        <DirectoryInput
          bind:value={cwd}
          Picker={pluginAvailable("files.upload")
            ? providers.DirectoryPicker
            : undefined}
        />
      </div>
    {/if}
    {#if pluginEnabled("files") && (!pluginAvailable("files.upload") || !providers.DirectoryPicker)}
      <p role="status" class="text-sm text-muted-foreground">
        文件功能暂不可用；仍可填写已有目录，但不能浏览或创建目录。
      </p>
    {/if}
    {#if pluginEnabled("chat") && (!pluginAvailable("chat.acp") || !providers.LaunchStartSideFields)}
      <p role="status" class="text-sm text-muted-foreground">
        聊天功能暂不可用；请选择终端启动配置或自定义命令。
      </p>
    {/if}
    {#if workspaceEnabled && fieldsRepository && providers.WorktreeCreateFields}
      <fieldset
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
        工作树已创建。再次提交只启动会话，不会重复创建分支或目录。
      </p>
    {/if}
    {#if workspaceEnabled && intent === "worktree"}
      <div class="flex items-center gap-2">
        <Checkbox id="worktree-with-session" bind:checked={withSession} />
        <Label for="worktree-with-session">创建后启动会话</Label>
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
              {pluginEnabled("launch")
                ? "启动配置界面暂不可用"
                : "启动配置插件未启用"}。请重新选择命令；不会从旧配置推测命令或环境变量。
            </p>
            <Button
              type="button"
              variant="outline"
              onclick={() => {
                source = "custom";
                onlaunchchoose();
              }}
            >
              使用自定义命令
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
      <AdvancedSection title="高级：会话名称" summary={name}>
        <div class="space-y-2">
          <Label for="session-name"
            >会话名称 <span class="text-muted-foreground">可选</span></Label
          ><Input
            id="session-name"
            bind:value={name}
            placeholder="留空则用首条提示，没有则跟随终端标题"
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
            onclick={onterminalcreate}>改为从终端开始</Button
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
          onclick={onclose}>取消</Button
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
