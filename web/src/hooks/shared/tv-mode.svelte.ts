import { deviceToggle } from "./device-preferences.svelte";

/** Display preference only: never part of the synchronized settings document. */
export const tvMode = deviceToggle("tv-mode", false);
