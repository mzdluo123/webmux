import { settings, changeSettings } from "../settings/settings.svelte";
import {
  SettingsValuesSchema,
  WorktreePinsSchema,
  type Repository,
  type SettingsValues,
  type Worktree,
  type WorktreePins,
} from "../../contracts/schemas";
import { homeLayout, setPinned } from "./home-worktrees";
import { homeWorktrees } from "./home-worktrees.svelte";
import {
  buildWorktreePinMenuProps,
  canMoveWorktreePin,
  isRepoWorktreePinsFull,
  isWorktreePinned,
  moveWorktreePin,
  removeRepositoryWorktreePins,
  removeWorktreePins,
  toggleWorktreePin,
} from "./worktree-pins";

export const worktreePins = {
  get value(): WorktreePins {
    return settings.values.worktreePins ?? {};
  },
  get ready(): boolean {
    return settings.ready;
  },
  has(repositoryId: string, worktreeId: string): boolean {
    return isWorktreePinned(this.value, repositoryId, worktreeId);
  },
  full(repo: Repository): boolean {
    return isRepoWorktreePinsFull(this.value, repo);
  },
  canMove(
    repo: Repository,
    worktreeId: string,
    offset: -1 | 1,
    candidates?: readonly Worktree[],
  ): boolean {
    return canMoveWorktreePin(this.value, repo, worktreeId, offset, candidates);
  },
  toggle(repo: Repository, worktreeId: string) {
    const current = this.value;
    const { pins: nextPins, pinned } = toggleWorktreePin(
      current,
      repo,
      worktreeId,
    );
    if (!pinned && nextPins === current) return;
    const changes: Partial<SettingsValues> = {
      worktreePins: WorktreePinsSchema.parse(nextPins),
    };
    if (pinned) {
      const shownOnHome = homeLayout(homeWorktrees.value, repo, []).shown.some(
        (item) => item.tree.id === worktreeId && item.pinned,
      );
      if (!shownOnHome) {
        const nextHome = SettingsValuesSchema.shape.homeWorktrees.safeParse(
          setPinned(homeWorktrees.value, repo, [worktreeId], true),
        );
        if (nextHome.success && nextHome.data) {
          changes.homeWorktrees = nextHome.data;
        }
      }
    }
    changeSettings(changes);
  },
  move(
    repo: Repository,
    worktreeId: string,
    offset: -1 | 1,
    candidates?: readonly Worktree[],
  ) {
    const next = moveWorktreePin(
      this.value,
      repo,
      worktreeId,
      offset,
      candidates,
    );
    if (next === this.value) return;
    changeSettings({ worktreePins: WorktreePinsSchema.parse(next) });
  },
  remove(repositoryId: string, removedIds: readonly string[]) {
    const next = removeWorktreePins(this.value, repositoryId, removedIds);
    if (next === this.value) return;
    changeSettings({ worktreePins: WorktreePinsSchema.parse(next) });
  },
  removeRepository(repositoryId: string) {
    const next = removeRepositoryWorktreePins(this.value, repositoryId);
    if (next === this.value) return;
    changeSettings({ worktreePins: WorktreePinsSchema.parse(next) });
  },
  menuProps(
    repo: Repository | undefined,
    worktreeId: string,
    candidates?: readonly Worktree[],
    pinsOverride?: WorktreePins,
  ) {
    return buildWorktreePinMenuProps({
      pins: pinsOverride ?? this.value,
      repo,
      worktreeId,
      candidates,
      ready: this.ready,
      ontoggle: (_, id) => {
        if (repo) this.toggle(repo, id);
      },
      onmove: (_, id, offset) => {
        if (repo) this.move(repo, id, offset, candidates);
      },
    });
  },
};
