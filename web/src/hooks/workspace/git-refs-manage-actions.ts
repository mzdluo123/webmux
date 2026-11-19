import { m } from "$lib/paraglide/messages.js";
import { SvelteSet } from "svelte/reactivity";
import { repositoryBranches, repositoryRemotes } from "../../platform/api";
import type {
  GitBranchRenamePreview,
  Repository,
  SessionInfo,
} from "../../contracts/schemas";
import type {
  GitBranchItem,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import { readFavoriteBranches } from "../../domains/workspace/git-favorites";
import { mapGitBranchesResponse } from "../../domains/workspace/git-workbench-adapter";

export function canConfirmBranchRename(options: {
  renameOpen: boolean;
  loadingPreview: boolean;
  submitting: boolean;
  hasPendingDebounce: boolean;
  oldName: string;
  newName: string;
  acknowledgeSessions: boolean;
  preview: GitBranchRenamePreview | null;
}): options is typeof options & { preview: GitBranchRenamePreview } {
  const cur = options.preview;
  const cleanNew = options.newName.trim();
  return Boolean(
    options.renameOpen &&
    !options.loadingPreview &&
    !options.submitting &&
    !options.hasPendingDebounce &&
    cur?.confirmationId &&
    cleanNew !== "" &&
    cleanNew !== options.oldName &&
    cur.oldName === options.oldName &&
    cur.newName === cleanNew &&
    cur.blockers.length === 0 &&
    (cur.sessions.length === 0 || options.acknowledgeSessions),
  );
}

export async function refreshRepositoryRefsAndRemotes(options: {
  repoId: string;
  isCurrentTarget: () => boolean;
  getRepository: () => Repository | undefined;
  getSessions: () => readonly SessionInfo[];
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onRefreshRepositories?: () => void;
  onAfterRefsChanged?: () => void;
}) {
  if (!options.repoId || !options.isCurrentTarget()) return;
  const [branchesRes, remotesRes] = await Promise.all([
    repositoryBranches(options.repoId).catch(() => null),
    repositoryRemotes(options.repoId).catch(() => null),
  ]);
  if (!options.isCurrentTarget()) return;
  const curView = options.getView();
  const patch: Partial<GitWorkbenchViewState> = {};
  if (branchesRes) {
    const localNames = new SvelteSet(branchesRes.branches.map((b) => b.name));
    const favorites = readFavoriteBranches(
      options.repoId,
      branchesRes.defaultBranch,
      localNames,
    );
    patch.branches = mapGitBranchesResponse(
      options.getRepository(),
      curView.context.worktreePath,
      options.getSessions(),
      branchesRes,
      favorites,
    );
  }
  if (remotesRes) {
    patch.remotes = remotesRes.remotes;
  }
  if (Object.keys(patch).length > 0) {
    options.patchView(patch);
  }
  options.onRefreshRepositories?.();
  options.onAfterRefsChanged?.();
}

export function resolveInitialUpstreamSelection(
  item: GitBranchItem,
  branches: readonly GitBranchItem[],
): string {
  const remoteItems = branches.filter((b) => b.kind === "remote");
  return (
    item.upstream ||
    remoteItems.find((r) => r.name.endsWith(`/${item.name}`))?.name ||
    remoteItems[0]?.name ||
    ""
  );
}

export function formatUpstreamUpdatedBanner(
  branch: string,
  upstream: string | undefined,
): string {
  return upstream
    ? m.s7_refs_upstream_set_banner({ branch, upstream })
    : m.s7_refs_upstream_unset_banner({ branch });
}

export function formatTagCreatedBanner(tag: {
  name: string;
  head: string;
  kind?: "lightweight" | "annotated";
}): string {
  const oid = tag.head.slice(0, 7);
  return tag.kind === "annotated"
    ? m.s7_refs_tag_created_annotated({ name: tag.name, oid })
    : m.s7_refs_tag_created_lightweight({ name: tag.name, oid });
}
