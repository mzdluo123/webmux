<script lang="ts">
  import { onDestroy, type Snippet } from "svelte";
  import IconClipboard from "~icons/tabler/clipboard";
  import IconKeyboard from "~icons/tabler/keyboard";
  import IconLock from "~icons/tabler/lock";
  import { Button } from "$lib/components/ui/button";
  import { Separator } from "$lib/components/ui/separator";
  import { m } from "$lib/paraglide/messages.js";
  import { createLongPress } from "../../hooks/shared/long-press";
  import {
    type Modifiers,
    createTerminalKeyRepeat,
    terminalSymbols as symbols,
    terminalLeadingKeys as leadingKeys,
    terminalArrowKeys as arrowKeys,
    terminalNavigationKeys as navigationKeys,
    terminalCombinations as combinations,
    terminalModifierKeys as modifierKeys,
  } from "../../domains/terminal/terminal-keys";

  let {
    send,
    modifiers,
    locked = false,
    toggle,
    lock = () => {},
    paste,
    keyboard,
    trailing,
  }: {
    send: (input: string) => void;
    modifiers: Modifiers;
    locked?: boolean;
    toggle: (key: keyof Modifiers) => void;
    lock?: (key: "shift") => void;
    paste: () => void;
    keyboard: () => void;
    trailing?: Snippet;
  } = $props();

  let fnActive = $state(false);
  let rowElement = $state<HTMLDivElement>();

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

  const arrowRepeat = createTerminalKeyRepeat((input) => send(input));
  const arrowHoldThresholdMs = 220;
  const arrowMoveLimitPx = 10;
  let arrowHoldTimer: ReturnType<typeof setTimeout> | undefined;
  let arrowPointerId: number | null = null;
  let arrowOriginX = 0;
  let arrowOriginY = 0;
  let arrowHeld = false;

  function stopArrowHold() {
    clearTimeout(arrowHoldTimer);
    arrowHoldTimer = undefined;
    arrowRepeat.stop();
    arrowPointerId = null;
  }

  function startArrowHold(input: string, event: PointerEvent) {
    if (
      !event.isPrimary ||
      (event.pointerType === "mouse" && event.button !== 0)
    )
      return;
    stopArrowHold();
    arrowHeld = false;
    arrowPointerId = event.pointerId;
    arrowOriginX = event.clientX;
    arrowOriginY = event.clientY;
    arrowHoldTimer = setTimeout(() => {
      arrowHoldTimer = undefined;
      arrowHeld = true;
      arrowRepeat.start(input);
    }, arrowHoldThresholdMs);
  }

  function moveArrowHold(event: PointerEvent) {
    if (arrowPointerId !== event.pointerId) return;
    if (
      Math.hypot(event.clientX - arrowOriginX, event.clientY - arrowOriginY) >
      arrowMoveLimitPx
    ) {
      stopArrowHold();
    }
  }

  function endArrowHold(event: PointerEvent) {
    if (arrowPointerId !== event.pointerId) return;
    stopArrowHold();
  }

  function clickArrow(input: string) {
    if (arrowHeld) {
      arrowHeld = false;
      return;
    }
    send(input);
  }

  function toggleFn() {
    stopArrowHold();
    fnActive = !fnActive;
    if (rowElement) rowElement.scrollLeft = 0;
  }

  function scrollRowWheel(event: WheelEvent) {
    if (event.deltaY === 0 || event.deltaX !== 0) return;
    const row = event.currentTarget as HTMLElement;
    if (row.scrollWidth <= row.clientWidth) return;
    event.preventDefault();
    row.scrollLeft += event.deltaY;
  }

  onDestroy(() => {
    shiftHold.cancel();
    stopArrowHold();
  });

  // iOS Safari drops the active element on pointerdown even when the event is
  // canceled if the element has a click handler, so cancel the compatibility
  // mousedown instead. Canceling pointerdown would suppress that mousedown and
  // leave the keyboard to close. Click still fires after a canceled mousedown.
  const keepFocus = (event: MouseEvent) => event.preventDefault();
</script>

{#snippet sendKey(label: string, input: string, hint?: string)}<Button
    variant="ghost"
    size="xs"
    title={hint}
    onmousedown={keepFocus}
    onclick={() => send(input)}>{label}</Button
  >{/snippet}

<aside
  class="soft-keys shrink-0"
  aria-label={m.s4_soft_keys_aria()}
  data-bottom-dock
>
  <div class="soft-keys-main">
    <div class="soft-leading">
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={m.s4_toggle_keyboard()}
        title={m.s4_toggle_keyboard()}
        onmousedown={keepFocus}
        onclick={keyboard}
        ><IconKeyboard class="size-4" aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="xs"
        aria-pressed={fnActive}
        title={m.s4_fn_keys_hint()}
        onmousedown={keepFocus}
        onclick={toggleFn}>Fn</Button
      >
    </div>
    <Separator orientation="vertical" class="mx-1 h-4 self-center" />
    <div class="soft-row" bind:this={rowElement} onwheel={scrollRowWheel}>
      {#if !fnActive}
        {#each leadingKeys as [label, input] (label)}{@render sendKey(
            label,
            input,
            label === "↵" ? m.s4_key_enter() : undefined,
          )}{/each}
        {#each modifierKeys as [key, label] (key)}
          <Button
            variant="ghost"
            size="xs"
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
        {#each arrowKeys as [label, input] (label)}
          <Button
            variant="ghost"
            size="xs"
            onmousedown={keepFocus}
            onpointerdown={(event) => startArrowHold(input, event)}
            onpointermove={moveArrowHold}
            onpointerup={endArrowHold}
            onpointercancel={endArrowHold}
            onpointerleave={endArrowHold}
            onlostpointercapture={stopArrowHold}
            onclick={() => clickArrow(input)}>{label}</Button
          >
        {/each}
      {:else}
        {#each symbols as symbol (symbol)}{@render sendKey(
            symbol,
            symbol,
          )}{/each}
        {#each navigationKeys as [label, input] (label)}{@render sendKey(
            label,
            input,
          )}{/each}
        {#each combinations as [label, input] (label)}{@render sendKey(
            label,
            input,
          )}{/each}
      {/if}
    </div>
    <Separator orientation="vertical" class="mx-1 h-4 self-center" />
    <div class="soft-trailing">
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={m.s4_paste()}
        title={m.s4_paste()}
        onmousedown={keepFocus}
        onclick={paste}
        ><IconClipboard class="size-4" aria-hidden="true" /></Button
      >
      {@render trailing?.()}
    </div>
  </div>
</aside>
