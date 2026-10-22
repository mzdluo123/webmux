<script lang="ts">
  import InlineNotice from "./InlineNotice.svelte";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Dialog from "$lib/components/ui/dialog";
  import {
    scratchEntriesLimit,
    scratchEntryLimit,
    scratchStorageLimit,
  } from "../draftStorage";
  import type {
    ScratchAddResult,
    ScratchTakeMode,
    ScratchTakeResult,
    TerminalDraft,
    TerminalDrafts,
  } from "../terminal-draft.svelte";

  let {
    open = $bindable(false),
    draft,
    drafts,
    saved,
  }: {
    open: boolean;
    draft: TerminalDraft;
    drafts: TerminalDrafts;
    // Where the scratch list is kept, e.g. "已保存在本机".
    saved: string;
  } = $props();
  let newText = $state("");
  let notice = $state("");
  let noticeTone = $state<"danger" | "success">("danger");
  let conflictId = $state<string | null>(null);
  let clearOpen = $state(false);
  const locked = $derived(!!draft.submission);
  const full = $derived(draft.scratch.length >= scratchEntriesLimit);
  const stale = $derived(
    !!draft.scratchGeneration &&
      !!draft.generation &&
      draft.scratchGeneration !== draft.generation,
  );

  function failure(result: ScratchAddResult | ScratchTakeResult) {
    if (result === "full")
      return `最多保留 ${scratchEntriesLimit} 条暂存，请先删除一条。`;
    if (result === "too-long")
      return `单条暂存最多 ${scratchEntryLimit.toLocaleString()} 个字符。`;
    if (result === "over-total")
      return `每会话暂存合计最多 ${scratchStorageLimit.toLocaleString()} 个字符。`;
    if (result === "locked")
      return "提交结果确认前，当前草稿不能转入或取用暂存。";
    if (result === "empty") return "暂存正文不能为空。";
    return "暂存操作失败。";
  }
  function add() {
    const result = drafts.addScratch(draft, newText);
    if (result === "added") {
      newText = "";
      if (!draft.scratchPersisted) {
        notice = "暂存已添加，但未能保存到本机，刷新后可能丢失。";
        noticeTone = "danger";
      } else notice = "";
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function stash() {
    const result = drafts.stash(draft);
    if (result === "added") {
      if (!draft.scratchPersisted) {
        notice = "当前草稿已存入暂存，但暂存未能保存到本机，刷新后可能丢失。";
        noticeTone = "danger";
      } else {
        notice = "当前草稿已存入暂存。";
        noticeTone = "success";
      }
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function take(id: string) {
    const item = draft.scratch.find((candidate) => candidate.id === id);
    if (!item?.text) {
      notice = "这条暂存没有正文。";
      noticeTone = "danger";
      return;
    }
    if (draft.submission) {
      notice = failure("locked");
      noticeTone = "danger";
      return;
    }
    if (draft.text) {
      conflictId = id;
      return;
    }
    takeNow(id, "replace");
  }
  function takeNow(id: string, mode: ScratchTakeMode) {
    const result = drafts.takeScratch(draft, id, mode);
    conflictId = null;
    if (result === "taken") {
      if (!draft.scratchPersisted || !draft.persisted) {
        notice =
          "已取用到当前草稿，但部分内容未能保存到本机，刷新后可能丢失；请保留当前页面内容。";
        noticeTone = "danger";
      } else {
        notice = "";
        open = false;
      }
    } else {
      notice = failure(result);
      noticeTone = "danger";
    }
  }
  function edit(id: string, value: string, element: HTMLTextAreaElement) {
    if (drafts.editScratch(draft, id, value)) {
      if (!draft.scratchPersisted) {
        notice = "暂存未能保存到本机，刷新后可能丢失。";
        noticeTone = "danger";
      } else if (notice) notice = "";
      return;
    }
    element.value =
      draft.scratch.find((candidate) => candidate.id === id)?.text ?? "";
    notice = `单条暂存最多 ${scratchEntryLimit.toLocaleString()} 个字符，且每会话合计最多 ${scratchStorageLimit.toLocaleString()} 个字符。`;
    noticeTone = "danger";
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>暂存</Dialog.Title>
      <Dialog.Description
        >暂存按会话保存在本机；取用只写入当前草稿，仍需点击发送，不会自动提交。</Dialog.Description
      >
    </Dialog.Header>
    {#if stale}
      <InlineNotice tone="warning"
        >这些暂存来自较早的会话实例。取用不会自动发送，请检查内容后再使用。</InlineNotice
      >
    {/if}
    {#if locked}
      <InlineNotice tone="info"
        >当前草稿有待确认的提交，转存和取用暂不可用；暂存仍可查看、编辑和删除。</InlineNotice
      >
    {/if}
    {#if notice}
      <InlineNotice tone={noticeTone}>{notice}</InlineNotice>
    {/if}
    <div class="flex flex-wrap items-center gap-2">
      <Button size="sm" disabled={!draft.text || locked || full} onclick={stash}
        >将当前草稿存入暂存</Button
      >
      <Button
        variant="outline"
        size="sm"
        disabled={!draft.scratch.length}
        onclick={() => (clearOpen = true)}>清空暂存</Button
      >
      <span class="text-xs text-muted-foreground">{saved}</span>
    </div>
    {#if draft.scratch.length}
      <div class="grid gap-3" role="list" aria-label="暂存列表">
        {#each draft.scratch as item, index (item.id)}
          <div class="grid gap-2 rounded-md border p-3" role="listitem">
            <Textarea
              value={item.text}
              aria-label={`暂存第 ${index + 1} 条`}
              rows={3}
              maxlength={scratchEntryLimit}
              class="max-h-[40dvh] font-mono"
              autocapitalize="off"
              autocorrect="off"
              spellcheck={false}
              oninput={(event) =>
                edit(item.id, event.currentTarget.value, event.currentTarget)}
            />
            <div
              class="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs text-muted-foreground"
            >
              <span>{new Date(item.updatedAt).toLocaleString()}</span>
              <span>{item.text.length} 字符</span>
              <div class="ml-auto flex flex-wrap gap-1">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={index === 0}
                  aria-label={`上移暂存第 ${index + 1} 条`}
                  onclick={() => drafts.moveScratch(draft, item.id, -1)}
                  >上移</Button
                >
                <Button
                  variant="outline"
                  size="sm"
                  disabled={index === draft.scratch.length - 1}
                  aria-label={`下移暂存第 ${index + 1} 条`}
                  onclick={() => drafts.moveScratch(draft, item.id, 1)}
                  >下移</Button
                >
                <Button
                  size="sm"
                  disabled={!item.text || locked}
                  aria-label={`取用暂存第 ${index + 1} 条`}
                  onclick={() => take(item.id)}>取用</Button
                >
                <Button
                  variant="outline"
                  size="sm"
                  aria-label={`删除暂存第 ${index + 1} 条`}
                  onclick={() => drafts.removeScratch(draft, item.id)}
                  >删除</Button
                >
              </div>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <p
        class="rounded-md border border-dashed p-4 text-sm text-muted-foreground"
      >
        还没有暂存。可以先记下稍后要交给这个会话的任务，再逐条取用。
      </p>
    {/if}
    <div class="grid gap-2 border-t pt-3">
      <Label for="scratch-new">新建暂存</Label>
      <Textarea
        id="scratch-new"
        bind:value={newText}
        rows={3}
        maxlength={scratchEntryLimit}
        placeholder="例如：稍后检查测试结果"
        class="max-h-[40dvh] font-mono"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
        aria-describedby="scratch-new-limits"
      />
      <div class="flex flex-wrap items-center gap-3">
        <Button size="sm" disabled={!newText || full} onclick={add}
          >添加暂存</Button
        >
        <span id="scratch-new-limits" class="text-xs text-muted-foreground"
          >最多 {scratchEntriesLimit} 条，单条 {scratchEntryLimit.toLocaleString()}
          字符，每会话合计 {scratchStorageLimit.toLocaleString()} 字符</span
        >
      </div>
    </div>
  </Dialog.Content>
</Dialog.Root>

<AlertDialog.Root
  open={conflictId !== null}
  onOpenChange={(value) => {
    if (!value) conflictId = null;
  }}
>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>当前草稿不为空</AlertDialog.Title>
      <AlertDialog.Description
        >可以将暂存追加到当前正文之后，或替换当前正文；替换前会把当前正文保存为一条新的暂存。</AlertDialog.Description
      >
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <Button
        variant="outline"
        onclick={() => conflictId && takeNow(conflictId, "append")}
        >追加到当前草稿</Button
      >
      <Button onclick={() => conflictId && takeNow(conflictId, "replace")}
        >替换当前草稿</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={clearOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>清空全部暂存？</AlertDialog.Title>
      <AlertDialog.Description
        >删除这个会话的全部暂存，不影响当前草稿、终端或已上传文件。此操作无法撤销。</AlertDialog.Description
      >
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <AlertDialog.Action
        variant="destructive"
        onclick={() => {
          drafts.clearScratch(draft);
          clearOpen = false;
        }}>确认清空</AlertDialog.Action
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
