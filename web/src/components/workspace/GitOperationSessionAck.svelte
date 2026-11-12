<script lang="ts">
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import type { SessionInfo } from "../../contracts/schemas";
  import WorktreeSessionList from "./WorktreeSessionList.svelte";

  let {
    sessions,
    acknowledgeSessions = false,
    disabled = false,
    acknowledgementLabel,
    onAcknowledgeSessionsChange,
  }: {
    sessions: readonly SessionInfo[];
    acknowledgeSessions?: boolean;
    disabled?: boolean;
    acknowledgementLabel: string;
    onAcknowledgeSessionsChange?: (checked: boolean) => void;
  } = $props();
</script>

{#if sessions.length > 0}
  <div class="space-y-2 rounded-lg border border-git-border p-3">
    <WorktreeSessionList
      title="当前工作树运行中的会话（不会关闭或重启）"
      {sessions}
    />
    <Label class="items-start">
      <Checkbox
        checked={acknowledgeSessions}
        onCheckedChange={(val) => onAcknowledgeSessionsChange?.(Boolean(val))}
        {disabled}
        class="mt-0.5"
      />
      <span class="text-sm leading-snug">{acknowledgementLabel}</span>
    </Label>
  </div>
{/if}
