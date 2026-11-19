<script lang="ts">
  import IconAlertCircle from "~icons/tabler/alert-circle";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import ConnectorInstallHint from "./ConnectorInstallHint.svelte";
  import {
    failedStep,
    type QuickStartTargetMode,
  } from "../../hooks/sessions/quick-start";
  import type {
    CreateSessionFailure,
    Repository,
  } from "../../contracts/schemas";
  import { openSession, type Shell } from "../../hooks/shared/shell.svelte";

  let {
    missing,
    targetMode,
    branchError,
    repository,
    failure,
    shell,
  }: {
    missing: "" | "agent" | "agent-uninstalled" | "repository";
    targetMode: QuickStartTargetMode;
    branchError?: string;
    repository?: Repository;
    failure?: CreateSessionFailure;
    shell: Shell;
  } = $props();
</script>

{#if missing}
  <div
    class="flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-sm"
    role="status"
  >
    <IconAlertCircle class="text-warning" aria-hidden="true" />
    <span class="mr-auto">
      {missing === "agent"
        ? m.s3_qs_missing_hybrid_config()
        : missing === "agent-uninstalled"
          ? m.s3_qs_all_hybrid_uninstalled()
          : m.s3_qs_missing_repository()}
    </span>
    {#if missing === "agent"}
      <Button variant="outline" size="sm-touch" href="/settings#launch-configs"
        >{m.s3_qs_new_launch_config()}</Button
      >
    {:else if missing === "agent-uninstalled"}
      <Button variant="outline" size="sm-touch" href="/settings#launch-configs"
        >{m.s3_qs_view_launch_configs()}</Button
      >
    {:else}
      <Button variant="outline" size="sm-touch" onclick={shell.addRepository}
        >{m.s3_qs_add_repository()}</Button
      >
    {/if}
  </div>
{/if}
{#if targetMode === "new" && branchError}
  <p role="alert" class="px-1 text-sm text-destructive">
    {m.s3_qs_read_branches_failed({
      name: repository?.name ?? "",
      error: branchError,
    })}
  </p>
{/if}
{#if failure}
  {@const failed = failure}
  <div
    class="space-y-2 rounded-lg border border-destructive/40 px-3 py-2 text-sm"
    role="alert"
  >
    <p class="flex items-start gap-2">
      <IconAlertCircle
        class="mt-0.5 shrink-0 text-destructive"
        aria-hidden="true"
      />
      <span class="min-w-0 break-words">
        <span class="font-medium"
          >{m.s3_qs_failed_step_prefix({ step: failedStep[failed.step] })}</span
        >{failed.error}
        {failed.sessionId ? "" : m.s3_qs_msg_not_sent_kept()}
      </span>
    </p>
    {#if failed.connector}
      <ConnectorInstallHint connector={failed.connector} />
    {/if}
    {#if failed.worktree || failed.sessionId}
      <p class="flex flex-wrap items-center gap-2 pl-6">
        {#if failed.worktree}
          {@const tree = failed.worktree}
          <span
            class="min-w-0 truncate font-mono text-xs text-muted-foreground"
            title={tree.path}
            >{m.s3_qs_created_worktree({ path: tree.path })}</span
          >
          {#if !failed.sessionId}
            <Button
              variant="outline"
              size="sm-touch"
              onclick={() =>
                shell.start({
                  kind: "worktree",
                  repositoryId: tree.repositoryId,
                  worktreeId: tree.id,
                })}>{m.s3_qs_new_session_in_worktree()}</Button
            >
          {/if}
        {/if}
        {#if failed.sessionId}
          {@const id = failed.sessionId}
          <Button
            variant="outline"
            size="sm-touch"
            onclick={() => openSession({ id })}
            >{m.s3_qs_enter_session()}</Button
          >
        {/if}
      </p>
    {/if}
  </div>
{/if}
