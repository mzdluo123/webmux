export { default as Menu } from "../components/settings/QuickCommandMenu.svelte";
export { quickCommands } from "../domains/settings/quick-commands.svelte";
export { executeSessionQuickCommand } from "../hooks/terminal/session-quick-command";
import { quickCommands } from "../domains/settings/quick-commands.svelte";
import {
  usesArtifactDir,
  usesWebmuxBrief,
} from "../domains/settings/quick-commands";

export function availableCommands(files: boolean) {
  return quickCommands.list.filter(
    (command) =>
      files ||
      (!usesArtifactDir(command.command) && !usesWebmuxBrief(command.command)),
  );
}
