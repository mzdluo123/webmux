import { hasTouchInput } from "../../platform/touch-input";
import { headsetMode } from "./headset-mode.svelte";
import { tvMode } from "./tv-mode.svelte";

/** Automatic restoration must not summon an IME or capture a TV remote. */
export function allowAutomaticInputFocus() {
  return !hasTouchInput() && !headsetMode.enabled && !tvMode.enabled;
}
