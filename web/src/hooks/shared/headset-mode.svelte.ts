import { deviceToggle } from "./device-preferences.svelte";

// A large coarse-pointer viewport can also be a tablet. Only an explicit,
// device-local choice may hide touch helpers or change controller focus policy.
export const headsetMode = deviceToggle("headsetMode", false);
