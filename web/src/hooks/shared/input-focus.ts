import { hasTouchInput } from "../../platform/touch-input";

/** Automatic restoration must not summon an IME. */
export function allowAutomaticInputFocus() {
  return !hasTouchInput();
}
