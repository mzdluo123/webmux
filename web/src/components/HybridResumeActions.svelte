<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import ConnectorInstallHint from "./ConnectorInstallHint.svelte";
  import HybridCandidatePicker from "./HybridCandidatePicker.svelte";
  import RebuildWorktreeDialog from "./RebuildWorktreeDialog.svelte";
  import { errorMessage } from "../error";
  import type { ResumeChoice, ResumeOutcome } from "../recoverable";
  import type {
    AgentConnector,
    HybridCandidate,
    HybridRecord,
    SessionInfo,
  } from "../schemas";

  // "恢复到聊天 / 恢复到终端" for one closed or interrupted hybrid session,
  // with the questions a resume may need answered first: rebuilding a
  // released worktree, another agent session when the recorded one is gone,
  // or resuming without a deleted launch config's environment.
  let {
    record,
    resume,
    onresumed,
    onrebuilt = () => {},
    disabled = false,
    onbusy = () => {},
    onerror,
  }: {
    record: HybridRecord;
    resume: (
      id: string,
      side: "tui" | "acp",
      choice?: ResumeChoice,
    ) => Promise<ResumeOutcome>;
    onresumed: (session: SessionInfo) => void;
    /**
     * The released worktree was rebuilt and the resume that followed is
     * settled, with every question it asked answered or dismissed: time to
     * refresh. `failure` explains a resume that failed ("" otherwise); it is
     * not reported through `onerror`, whose display a refresh may replace.
     */
    onrebuilt?: (failure: string) => void;
    disabled?: boolean;
    onbusy?: (busy: boolean) => void;
    /** Reports a failed resume; "" clears it when a new attempt starts. */
    onerror: (message: string) => void;
  } = $props();

  // A resume that needs the user's answer before it can be retried. Earlier
  // answers ride along, so a second question does not undo the first.
  let pending = $state<{
    side: "tui" | "acp";
    choice: ResumeChoice;
    kind: "candidates" | "config_missing";
    message: string;
    candidates: HybridCandidate[];
  } | null>(null);
  let pendingOpen = $state(false);
  let picked = $state("");

  // Resumes in flight; the refresh after a rebuild waits for them.
  let running = 0;
  async function run(side: "tui" | "acp", choice: ResumeChoice = {}) {
    running++;
    onbusy(true);
    onerror("");
    try {
      const outcome = await resume(record.id, side, choice);
      if (outcome.status === "resumed") {
        onresumed(outcome.session);
        return;
      }
      if (outcome.status === "connector_missing") {
        missing = { connector: outcome.connector, choice };
        missingOpen = true;
        return;
      }
      pending = {
        side,
        choice,
        kind: outcome.status,
        message: outcome.message,
        candidates: outcome.status === "candidates" ? outcome.candidates : [],
      };
      picked = pending.candidates[0]?.agentSessionId ?? "";
      pendingOpen = true;
    } catch (err) {
      const message = `无法恢复“${record.title || record.name}”：${errorMessage(err)}`;
      if (rebuild) rebuild.failure = message;
      else onerror(message);
    } finally {
      running--;
      onbusy(false);
      settle();
    }
  }

  function confirmPending() {
    const answer = pending;
    pendingOpen = false;
    if (!answer) return;
    void run(
      answer.side,
      answer.kind === "candidates"
        ? { ...answer.choice, agentSessionId: picked }
        : { ...answer.choice, confirmWithoutEnv: true },
    );
  }

  // The chat side's connector is not installed; the terminal side still
  // works, with the answers given so far.
  let missing = $state<{
    connector: AgentConnector;
    choice: ResumeChoice;
  } | null>(null);
  let missingOpen = $state(false);

  function resumeTerminal() {
    const answer = missing;
    missingOpen = false;
    if (answer) void run("tui", answer.choice);
  }

  // A released record's worktree comes back first, once the user confirms
  // how. The record shown keeps its release until the records refresh, so
  // the release rebuilt here is remembered and not rebuilt twice.
  let rebuildSide = $state<"tui" | "acp" | null>(null);
  let rebuiltRelease = "";
  function start(side: "tui" | "acp") {
    if (record.released && record.released.id !== rebuiltRelease)
      rebuildSide = side;
    else void run(side);
  }
  // The refresh after a rebuild can regroup the record and so replace this
  // component; it waits until the resume and every question it asked are
  // settled, and hands a failure to the caller, which outlives the regroup.
  let rebuild: { failure: string } | null = null;
  function rebuilt() {
    const side = rebuildSide;
    rebuildSide = null;
    rebuiltRelease = record.released?.id ?? "";
    rebuild = { failure: "" };
    if (side) void run(side);
    else settle();
  }
  function settle() {
    if (!rebuild || running || pendingOpen || missingOpen) return;
    const { failure } = rebuild;
    rebuild = null;
    onrebuilt(failure);
  }
</script>

<Button size="sm" {disabled} onclick={() => start("acp")}>恢复到聊天</Button>
<Button size="sm" variant="outline" {disabled} onclick={() => start("tui")}
  >恢复到终端</Button
>

{#if rebuildSide && record.released}<RebuildWorktreeDialog
    id={record.id}
    title={record.title || record.name}
    released={record.released}
    onclose={() => (rebuildSide = null)}
    onrebuilt={rebuilt}
  />{/if}

<ConfirmDialog
  bind:open={
    () => pendingOpen,
    (open) => {
      pendingOpen = open;
      settle();
    }
  }
  title={pending?.kind === "candidates"
    ? "找不到原来的 agent 会话"
    : "启动配置已删除"}
  onconfirm={confirmPending}
>
  {pending?.message}
  {#if pending?.kind === "candidates"}
    <HybridCandidatePicker
      candidates={pending.candidates}
      bind:picked
      label="选择要恢复的会话"
    />
  {:else}
    恢复后将使用原命令，但不再带有该配置保存的环境变量。
  {/if}
</ConfirmDialog>

<AlertDialog.Root
  bind:open={
    () => missingOpen,
    (open) => {
      missingOpen = open;
      settle();
    }
  }
>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>聊天端无法启动</AlertDialog.Title>
      <AlertDialog.Description
        >可以先安装后再恢复到聊天，或者恢复到终端。</AlertDialog.Description
      >
    </AlertDialog.Header>
    {#if missing}<ConnectorInstallHint connector={missing.connector} />{/if}
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <AlertDialog.Action onclick={resumeTerminal}
        >恢复到终端</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
