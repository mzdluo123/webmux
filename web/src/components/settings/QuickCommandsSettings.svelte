<script lang="ts">
  import { tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { QuickCommand } from "../../contracts/schemas";
  import {
    QUICK_COMMANDS_LIMIT,
    commandLabel,
    commandPreview,
  } from "../../domains/settings/quick-commands";
  import {
    quickCommands,
    saveQuickCommands,
  } from "../../domains/settings/quick-commands.svelte";
  import QuickCommandDialog from "./QuickCommandDialog.svelte";
  import QuickCommandPresetMenu from "./QuickCommandPresetMenu.svelte";
  import {
    settings,
    changeSettings,
    saveSettings,
  } from "../../domains/settings/settings.svelte";
  import { errorMessage } from "../../platform/error";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconBolt from "~icons/tabler/bolt";
  import IconPencil from "~icons/tabler/pencil";
  import IconPlus from "~icons/tabler/plus";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { InfoHint } from "$lib/components/ui/info-hint";
  import SettingsRowMenu from "./SettingsRowMenu.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import * as Card from "$lib/components/ui/card";
  import * as Empty from "$lib/components/ui/empty";
  import * as Item from "$lib/components/ui/item";

  let dialogOpen = $state(false);
  let editing = $state<QuickCommand>();
  let deleteOpen = $state(false);
  let clearQuickOpen = $state(false);
  let deleting = $state<QuickCommand>();
  let presetBusy = $state(false);
  let presetError = $state("");
  let lastAddedId = $state<string>();
  let locateButton = $state<HTMLButtonElement | null>(null);
  const full = $derived(quickCommands.list.length >= QUICK_COMMANDS_LIMIT);
  const lastAdded = $derived(
    quickCommands.list.find((item) => item.id === lastAddedId),
  );

  function edit(cmd?: QuickCommand) {
    editing = cmd;
    dialogOpen = true;
  }

  async function upsert(cmd: QuickCommand) {
    const list = quickCommands.list;
    const exists = list.some((item) => item.id === cmd.id);
    if (!exists && list.length >= QUICK_COMMANDS_LIMIT)
      return m.s2_qc_err_limit({ limit: String(QUICK_COMMANDS_LIMIT) });
    await saveQuickCommands(
      exists
        ? list.map((item) => (item.id === cmd.id ? cmd : item))
        : [...list, cmd],
    );
    if (!exists) lastAddedId = cmd.id;
    return "";
  }

  async function addPresets(presets: QuickCommand[]) {
    presetBusy = true;
    presetError = "";
    try {
      await saveQuickCommands([...quickCommands.list, ...presets]);
      lastAddedId = presets[0]?.id;
    } catch (err) {
      presetError = errorMessage(err);
    } finally {
      presetBusy = false;
    }
    await tick();
    if (document.activeElement === document.body)
      locateButton?.focus({ preventScroll: true });
  }

  function locateAdded() {
    if (!lastAdded) return;
    const target = document.getElementById(`quick-command-${lastAdded.id}`);
    target?.focus({ preventScroll: true });
    target?.scrollIntoView({ block: "nearest" });
  }

  function move(index: number, offset: -1 | 1) {
    const list = [...quickCommands.list];
    const from = list[index];
    const to = list[index + offset];
    if (!from || !to) return;
    list[index] = to;
    list[index + offset] = from;
    void saveQuickCommands(list).catch(() => undefined);
  }
</script>

<Card.Root flat id="quick-commands" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_qc_title()}</Card.Title>
    <Card.Description
      ><span class="flex items-center gap-1"
        >{m.s2_qc_desc()}<InfoHint label={m.s2_qc_title()}
          >{m.s2_qc_hint_1()}<IconBolt
            class="inline size-3.5 align-[-2px]"
            aria-label={m.s2_bolt_icon_aria()}
          />{m.s2_qc_hint_2()}</InfoHint
        ></span
      ></Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <Button
        variant="outline"
        disabled={!settings.ready || full}
        onclick={() => edit()}
        ><IconPlus aria-hidden="true" />{m.s2_qc_add_title()}</Button
      >
      <QuickCommandPresetMenu
        commands={quickCommands.list}
        busy={!settings.ready || presetBusy}
        onadd={(presets) => void addPresets(presets)}
      />
      <span class="text-xs text-muted-foreground" aria-live="polite"
        >{m.s2_qc_limit_short({
          limit: String(QUICK_COMMANDS_LIMIT),
          count: String(quickCommands.list.length),
        })}</span
      >
    </div>
    <div class="min-h-(--control-sm)" aria-live="polite">
      {#if lastAdded}
        <Button
          bind:ref={locateButton}
          variant="link"
          size="sm"
          wrap
          data-quick-command-locate
          onclick={locateAdded}
          >{m.s2_qc_locate_added({ name: commandLabel(lastAdded) })}</Button
        >
      {/if}
    </div>
    {#if quickCommands.list.length === 0}
      <Empty.Root variant="outline">
        <Empty.Header>
          <Empty.Media variant="icon"
            ><IconBolt aria-hidden="true" /></Empty.Media
          >
          <Empty.Title>{m.s2_qc_empty_title()}</Empty.Title>
          <Empty.Description>{m.s2_qc_empty_desc()}</Empty.Description>
        </Empty.Header>
      </Empty.Root>
    {:else}
      <Item.Group variant="divided" aria-label={m.s2_qc_list_aria()}>
        {#each quickCommands.list as cmd, index (cmd.id)}
          {@const label = commandLabel(cmd)}
          <Item.Root variant="divided" size="flush" class="min-h-14">
            <ListButton
              id={`quick-command-${cmd.id}`}
              variant="content"
              class="flex min-w-0 flex-1 items-center"
              aria-label={m.s2_edit_item({ name: label })}
              onclick={() => edit(cmd)}
            >
              <span class="flex min-w-0 flex-1 items-center gap-2">
                <span class="min-w-0 truncate text-sm font-medium">{label}</span
                >
                {#if cmd.enter}<Badge variant="secondary" class="shrink-0"
                    >{m.s2_qc_enter_badge()}</Badge
                  >{/if}
                {#if cmd.name}<span
                    class="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground"
                    >{commandPreview(cmd)}</span
                  >{/if}
              </span>
            </ListButton>
            <SettingsRowMenu
              {label}
              disabled={!settings.ready}
              ondelete={() => {
                deleting = cmd;
                deleteOpen = true;
              }}
            >
              <DropdownMenu.Item
                aria-label={m.s2_edit_item({ name: label })}
                onSelect={() => edit(cmd)}
                ><IconPencil
                  aria-hidden="true"
                />{m.s2_edit()}</DropdownMenu.Item
              >
              <DropdownMenu.Item
                aria-label={m.s2_move_up_item({ name: label })}
                disabled={index === 0}
                onSelect={() => move(index, -1)}
                ><IconArrowUp
                  aria-hidden="true"
                />{m.s2_move_up()}</DropdownMenu.Item
              >
              <DropdownMenu.Item
                aria-label={m.s2_move_down_item({ name: label })}
                disabled={index === quickCommands.list.length - 1}
                onSelect={() => move(index, 1)}
                ><IconArrowDown
                  aria-hidden="true"
                />{m.s2_move_down()}</DropdownMenu.Item
              >
            </SettingsRowMenu>
          </Item.Root>
        {/each}
      </Item.Group>
    {/if}
    {#if presetError}
      <InlineNotice tone="danger">
        <p>{m.s2_qc_preset_err({ error: presetError })}</p>
      </InlineNotice>
    {/if}
    <div class="flex flex-wrap gap-2">
      <Button
        variant="outline"
        size="sm"
        disabled={!settings.ready}
        onclick={() => changeSettings({ quickCommands: null })}
        >{m.s2_qc_restore_presets()}</Button
      >
      <Button
        variant="destructive-outline"
        size="sm"
        disabled={!settings.ready || quickCommands.list.length === 0}
        onclick={() => {
          clearQuickOpen = true;
        }}>{m.s2_qc_clear_all_btn()}</Button
      >
    </div>
  </Card.Content>
</Card.Root>

<QuickCommandDialog
  bind:open={dialogOpen}
  initial={editing}
  existing={quickCommands.list}
  save={upsert}
/>
<AlertDialog.Root bind:open={deleteOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s2_qc_delete_title()}</AlertDialog.Title>
      <AlertDialog.Description
        >{m.s2_qc_delete_desc({
          label: deleting ? commandLabel(deleting) : "",
        })}</AlertDialog.Description
      >
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{m.s2_cancel()}</AlertDialog.Cancel>
      <Button
        variant="destructive"
        onclick={async () => {
          try {
            await saveQuickCommands(
              quickCommands.list.filter((item) => item.id !== deleting?.id),
            );
            deleteOpen = false;
          } catch {
            /* shared settings status retains error */
          }
        }}>{m.s2_delete()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
<AlertDialog.Root bind:open={clearQuickOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{m.s2_qc_clear_title()}</AlertDialog.Title>
      <AlertDialog.Description>{m.s2_qc_clear_desc()}</AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>{m.s2_cancel()}</AlertDialog.Cancel>
      <Button
        variant="destructive"
        disabled={!settings.ready || settings.saving}
        onclick={async () => {
          try {
            await saveSettings({ quickCommands: [] });
            clearQuickOpen = false;
          } catch {
            /* shared status */
          }
        }}>{m.s2_delete_all()}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
