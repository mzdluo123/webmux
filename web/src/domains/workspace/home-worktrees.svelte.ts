import { settings, changeSettings } from "../settings/settings.svelte";
import { store, stored } from "../../platform/storage";
import {
  HomeWorktreesSchema,
  type HomeWorktrees,
  type Repository,
} from "../../contracts/schemas";
import {
  parseHomeWorktrees,
  reconcileDiscoveredWorktrees,
} from "./home-worktrees";

const KNOWN_WORKTREES_KEY = "knownWorktrees";

export const homeWorktrees = {
  get value(): HomeWorktrees {
    return settings.values.homeWorktrees ?? {};
  },
  save(next: HomeWorktrees) {
    changeSettings({ homeWorktrees: HomeWorktreesSchema.parse(next) });
  },
  syncDiscovered(
    repositories: readonly Repository[],
    settled: boolean,
    error: string,
  ) {
    if (!settings.ready || !settled || error) return;
    const known = parseHomeWorktrees(stored(KNOWN_WORKTREES_KEY));
    const currentHome = settings.values.homeWorktrees ?? {};
    const reconciled = reconcileDiscoveredWorktrees({
      known,
      home: currentHome,
      repositories,
      settled,
      error,
    });
    if (reconciled.known !== known) {
      store(KNOWN_WORKTREES_KEY, JSON.stringify(reconciled.known));
    }
    if (reconciled.home !== currentHome) {
      changeSettings({
        homeWorktrees: HomeWorktreesSchema.parse(reconciled.home),
      });
    }
  },
};
