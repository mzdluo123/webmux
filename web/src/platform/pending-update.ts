import { z } from "zod";
import * as m from "$lib/paraglide/messages.js";
import { store, stored } from "./api";
import type { PendingUpdate } from "../contracts/schemas";

// How the user restarts WebMux for the recorded launcher. Restarting is never
// offered as a button: it ends every session.
export type RestartMethod = { command: string } | { text: string };

export function restartMethod(
  launcher: PendingUpdate["launcher"],
): RestartMethod {
  switch (launcher) {
    case "systemd":
      return { command: "systemctl --user restart webmux" };
    case "launchd":
      return { command: "webmux service restart" };
    case "windows-tray":
      return { text: m.s1_restart_via_tray() };
    case "manual":
      return { text: m.s1_restart_manual() };
  }
}

// The home page notice dismissal is device-local and remembers one version,
// so a newer installed version shows the notice again.
const dismissedKey = "dismissedUpdate";
const DismissedUpdateSchema = z.string().min(1).max(64);

export function dismissedUpdate(): string {
  return DismissedUpdateSchema.safeParse(stored(dismissedKey)).data ?? "";
}

export function dismissUpdate(version: string) {
  store(dismissedKey, version);
}

// The home page notice: none without a pending update or once its version
// was dismissed. The settings page always shows a pending update.
export function homeUpdate(
  update: PendingUpdate | undefined,
  dismissed: string,
): PendingUpdate | undefined {
  return update && update.installedVersion !== dismissed ? update : undefined;
}
