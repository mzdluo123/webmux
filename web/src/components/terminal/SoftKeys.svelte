<script lang="ts">
  import { onDestroy, type Snippet } from "svelte";
  import IconClipboard from "~icons/tabler/clipboard";
  import IconKeyboard from "~icons/tabler/keyboard";
  import IconLock from "~icons/tabler/lock";
  import { Button } from "$lib/components/ui/button";
  import { Separator } from "$lib/components/ui/separator";
  import * as m from "$lib/paraglide/messages.js";
  import { createLongPress } from "../../hooks/shared/long-press";
  import {
    type Modifiers,
    terminalSymbols as symbols,
    terminalLeadingKeys as leadingKeys,
    terminalArrowKeys as arrowKeys,
    terminalNavigationKeys as navigationKeys,
    terminalCombinations as combinations,
    terminalModifierKeys as modifierKeys,
  } from "../../domains/terminal/terminal-keys";
  import TouchGamepad from "./TouchGamepad.svelte";

  let {
    send,
    modifiers,
    locked = false,
    gamepad = false,
    toggle,
    lock = () => {},
    paste,
    keyboard,
    gamepadToggle,
    trailing,
  }: {
    send: (input: string) => void;
    modifiers: Modifiers;
    locked?: boolean;
    gamepad?: boolean;
    toggle: (key: keyof Modifiers) => void;
    lock?: (key: "shift") => void;
    paste: () => void;
    keyboard: () => void;
    gamepadToggle?: Snippet;
    trailing?: Snippet;
  } = $props();
  const visibleCombinations = $derived(
    gamepad
      ? combinations.filter(([label]) => label !== "Ctrl+C")
      : combinations,
  );
  const shiftTitle = $derived(
    locked ? m.s4_shift_locked_hint() : m.s4_shift_unlocked_hint(),
  );
  const shiftHold = createLongPress();
  // A long press locks Shift; the click that follows the release must not
  // toggle it back off.
  let held = false;
  function startHold(key: keyof Modifiers, event: PointerEvent) {
    if (key !== "shift") return;
    held = false;
    shiftHold.start(event, () => {
      held = true;
      lock("shift");
    });
  }
  function stopHold(key: keyof Modifiers) {
    if (key === "shift") shiftHold.cancel();
  }
  onDestroy(shiftHold.cancel);
  // iOS Safari drops the active element on pointerdown even when the event is
  // canceled if the element has a click handler, so cancel the compatibility
  // mousedown instead. Canceling pointerdown would suppress that mousedown and
  // leave the keyboard to close. Click still fires after a canceled mousedown.
  const keepFocus = (event: MouseEvent) => event.preventDefault();
</script>

{#snippet sendKey(label: string, input: string)}<Button
    variant="ghost"
    size="sm"
    onmousedown={keepFocus}
    onclick={() => send(input)}>{label}</Button
  >{/snippet}

<aside
  class="soft-keys shrink-0 border-t bg-card"
  aria-label={m.s4_soft_keys_aria()}
>
  {#if gamepad}<TouchGamepad {send} />{/if}
  <div class="soft-keys-main">
    <div class="soft-row">
      <Button
        variant="ghost"
        size="sm"
        aria-label={m.s4_toggle_keyboard()}
        title={m.s4_toggle_keyboard()}
        onmousedown={keepFocus}
        onclick={keyboard}><IconKeyboard aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="sm"
        aria-label={m.s4_paste()}
        title={m.s4_paste()}
        onmousedown={keepFocus}
        onclick={paste}><IconClipboard aria-hidden="true" /></Button
      >
      {@render gamepadToggle?.()}
      {#each symbols as symbol (symbol)}
        <Button
          variant="ghost"
          size="sm"
          onmousedown={keepFocus}
          onclick={() => send(symbol)}>{symbol}</Button
        >
      {/each}
      {#if !gamepad}
        {#each leadingKeys as [label, input] (label)}{@render sendKey(
            label,
            input,
          )}{/each}
      {/if}
      {#each modifierKeys as [key, label] (key)}
        <Button
          variant="ghost"
          size="sm"
          aria-pressed={modifiers[key]}
          title={key === "shift" ? shiftTitle : undefined}
          onmousedown={keepFocus}
          onpointerdown={(event) => startHold(key, event)}
          onpointermove={shiftHold.move}
          onpointerup={() => stopHold(key)}
          onpointercancel={() => stopHold(key)}
          onpointerleave={() => stopHold(key)}
          onclick={() => {
            if (key === "shift" && held) {
              held = false;
              return;
            }
            toggle(key);
          }}
          >{label}{#if key === "shift" && locked}<IconLock
              aria-hidden="true"
            />{/if}</Button
        >
      {/each}
      {#if !gamepad}
        {#each arrowKeys as [label, input] (label)}{@render sendKey(
            label,
            input,
          )}{/each}
      {/if}
      {#each navigationKeys as [label, input] (label)}{@render sendKey(
          label,
          input,
        )}{/each}
      {#each visibleCombinations as [label, input] (label)}{@render sendKey(
          label,
          input,
        )}{/each}
    </div>
    {#if trailing}
      <Separator orientation="vertical" class="mx-1.5 h-6 self-center" />
      <div class="soft-trailing">{@render trailing()}</div>
    {/if}
  </div>
</aside>
