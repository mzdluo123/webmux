<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import * as Item from "$lib/components/ui/item";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import InlineNotice from "./InlineNotice.svelte";
  import { errorMessage } from "../error";
  import type { ResumeChoice, ResumeOutcome } from "../recoverable";
  import type { HybridCandidate, HybridRecord, SessionInfo } from "../schemas";

  let {
    records,
    now,
    resume,
    remove,
    onresumed,
  }: {
    records: HybridRecord[];
    now: number;
    resume: (
      id: string,
      side: "tui" | "acp",
      choice?: ResumeChoice,
    ) => Promise<ResumeOutcome>;
    remove: (id: string) => Promise<unknown>;
    onresumed: (session: SessionInfo) => void;
  } = $props();

  let expanded = $state(false);
  let busy = $state("");
  let error = $state("");
  let removing = $state<HybridRecord | null>(null);
  let removeOpen = $state(false);
  // A resume that needs the user's answer before it can be retried.
  let pending = $state<{
    record: HybridRecord;
    side: "tui" | "acp";
    kind: "candidates" | "config_missing";
    message: string;
    candidates: HybridCandidate[];
  } | null>(null);
  let pendingOpen = $state(false);
  let picked = $state("");

  function since(record: HybridRecord) {
    const at = Date.parse(record.interruptedAt ?? record.updatedAt);
    const seconds = Math.max(0, Math.floor((now - at) / 1000));
    if (seconds < 60) return "刚刚";
    if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`;
    return `${Math.floor(seconds / 86400)} 天前`;
  }

  async function run(
    record: HybridRecord,
    side: "tui" | "acp",
    choice: ResumeChoice = {},
  ) {
    busy = record.id;
    error = "";
    try {
      const outcome = await resume(record.id, side, choice);
      if (outcome.status === "resumed") {
        onresumed(outcome.session);
        return;
      }
      pending = {
        record,
        side,
        kind: outcome.status,
        message: outcome.message,
        candidates: outcome.status === "candidates" ? outcome.candidates : [],
      };
      picked = pending.candidates[0]?.agentSessionId ?? "";
      pendingOpen = true;
    } catch (err) {
      error = `无法恢复“${record.name}”：${errorMessage(err)}`;
    } finally {
      busy = "";
    }
  }

  function confirmPending() {
    const answer = pending;
    pendingOpen = false;
    if (!answer) return;
    void run(
      answer.record,
      answer.side,
      answer.kind === "candidates"
        ? { agentSessionId: picked }
        : { confirmWithoutEnv: true },
    );
  }

  async function confirmRemove() {
    const record = removing;
    removeOpen = false;
    if (!record) return;
    busy = record.id;
    error = "";
    try {
      await remove(record.id);
    } catch (err) {
      error = `无法移除“${record.name}”：${errorMessage(err)}`;
    } finally {
      busy = "";
    }
  }
</script>

<section class="min-w-0 space-y-3" aria-label="可恢复" hidden={!records.length}>
  <Collapsible.Root bind:open={expanded}>
    <Collapsible.Trigger>
      {#snippet child({ props })}
        <Button {...props} variant="ghost" size="sm" class="-ml-2">
          <IconChevronDown
            aria-hidden="true"
            class={expanded ? "" : "-rotate-90"}
          />
          <span class="font-semibold">可恢复</span>
          <Badge variant="secondary">{records.length}</Badge>
        </Button>
      {/snippet}
    </Collapsible.Trigger>
    <p class="mt-1 text-xs text-muted-foreground">
      已关闭或已中断的混合会话；对话保存在 agent 中，可以恢复到聊天或终端。
    </p>
    <Collapsible.Content class="mt-3 grid min-w-0 gap-3">
      {#if error}<InlineNotice tone="danger">{error}</InlineNotice>{/if}
      {#each records as record (record.id)}
        <Item.Root variant="outline" class="min-w-0 flex-col items-stretch">
          <Item.Content class="min-w-0">
            <Item.Title class="min-w-0">
              <span class="truncate">{record.name}</span>
              <Badge variant="outline"
                >{record.state === "closed" ? "已关闭" : "已中断"}</Badge
              >
            </Item.Title>
            <Item.Description>
              <span class="flex flex-wrap gap-x-3 gap-y-1">
                <span>{record.agent}</span>
                <span class="inline-flex items-center gap-1"
                  >{#if record.side === "acp"}<IconMessageCircle
                      aria-hidden="true"
                    />聊天{:else}<IconTerminal2
                      aria-hidden="true"
                    />终端{/if}</span
                >
                <span>{since(record)}</span>
              </span>
            </Item.Description>
            <p class="font-mono text-xs break-all text-muted-foreground">
              {record.cwd}
            </p>
          </Item.Content>
          <Item.Actions class="flex flex-wrap gap-2">
            <Button
              size="sm"
              disabled={!!busy}
              onclick={() => run(record, "acp")}>恢复到聊天</Button
            >
            <Button
              size="sm"
              variant="outline"
              disabled={!!busy}
              onclick={() => run(record, "tui")}>恢复到终端</Button
            >
            <Button
              size="sm"
              variant="ghost"
              disabled={!!busy}
              onclick={() => {
                removing = record;
                removeOpen = true;
              }}>移除</Button
            >
          </Item.Actions>
        </Item.Root>
      {/each}
    </Collapsible.Content>
  </Collapsible.Root>
</section>

<ConfirmDialog
  bind:open={removeOpen}
  title={`移除“${removing?.name ?? ""}”？`}
  onconfirm={confirmRemove}
>
  只移除 WebMux 的记录与本机草稿；agent 自己保存的会话不受影响。
</ConfirmDialog>

<ConfirmDialog
  bind:open={pendingOpen}
  title={pending?.kind === "candidates"
    ? "找不到原来的 agent 会话"
    : "启动配置已删除"}
  onconfirm={confirmPending}
>
  {pending?.message}
  {#if pending?.kind === "candidates"}
    {#if pending.candidates.length}
      <NativeSelect.Root
        aria-label="选择要恢复的会话"
        class="mt-3 w-full"
        bind:value={picked}
      >
        {#each pending.candidates as candidate (candidate.agentSessionId)}
          <NativeSelect.Option value={candidate.agentSessionId}
            >{candidate.title || candidate.agentSessionId}</NativeSelect.Option
          >
        {/each}
      </NativeSelect.Root>
    {:else}
      同一目录下没有其他可恢复的会话。
    {/if}
  {:else}
    恢复后将使用原命令，但不再带有该配置保存的环境变量。
  {/if}
</ConfirmDialog>
