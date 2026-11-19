import { m } from "$lib/paraglide/messages.js";
import type {
  GitWorkbenchViewState,
  GitWorkingChange,
} from "../../contracts/git-workbench-schemas";
import type {
  GitLineSelection,
  GitStagingEntry,
} from "../../contracts/schemas";
import type { RequestTicket } from "../shared/remote-request.svelte";
import { performStageMutation } from "./git-workbench-actions";

interface StageMutationHost {
  kind: "stage" | "unstage";
  repoId: string;
  wtId: string;
  req: RequestTicket;
  isSameTarget: () => boolean;
  onRecordStagedPath: (path: string) => void;
  onReloadAfterMutation: (signal: AbortSignal) => Promise<void>;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onRefreshRepositories?: () => void;
  setStagingBusy: (busy: boolean) => void;
}

async function withGuardedStageRun(
  host: StageMutationHost,
  run: (active: () => boolean) => Promise<void>,
) {
  const active = () =>
    host.req.current() && !host.req.signal.aborted && host.isSameTarget();
  host.setStagingBusy(true);
  try {
    await run(active);
  } finally {
    if (host.req.current() && host.isSameTarget()) {
      host.setStagingBusy(false);
    }
  }
}

export async function executeSingleStageMutation(
  options: StageMutationHost & {
    entry: GitStagingEntry;
    hunkId?: string;
    lineSelections?: readonly GitLineSelection[];
    onMarkStale: (reason: string) => void;
  },
) {
  await withGuardedStageRun(options, async (active) => {
    const out = await performStageMutation({
      kind: options.kind,
      repositoryId: options.repoId,
      worktreeId: options.wtId,
      entry: options.entry,
      hunkId: options.hunkId,
      lineSelections: options.lineSelections,
      signal: options.req.signal,
    });
    if (!active()) return;
    if (out.ok) {
      if (options.kind === "stage") {
        options.onRecordStagedPath(options.entry.path);
      }
      await options.onReloadAfterMutation(options.req.signal);
      if (!active()) return;
      options.onRefreshRepositories?.();
    } else if (out.reason === "stale") {
      const reason = out.banner || m.s7_staging_external_stale_stage();
      options.onMarkStale(reason);
      options.patchView({ statusBanner: reason });
    } else if (out.busyOutcome) {
      options.patchView({ lastCommitOutcome: out.busyOutcome });
    } else if (out.banner) {
      options.patchView({ statusBanner: out.banner });
    }
  });
}

export async function executeGroupStageMutation(
  options: StageMutationHost & {
    targets: readonly GitWorkingChange[];
    getEntry: (id: string) => GitStagingEntry | undefined;
  },
) {
  await withGuardedStageRun(options, async (active) => {
    let failureBanner = "";
    for (const item of options.targets) {
      if (!active()) return;
      const entry = options.getEntry(item.id);
      if (!entry) continue;
      const out = await performStageMutation({
        kind: options.kind,
        repositoryId: options.repoId,
        worktreeId: options.wtId,
        entry,
        signal: options.req.signal,
      });
      if (!active()) return;
      if (out.ok) {
        if (options.kind === "stage") options.onRecordStagedPath(entry.path);
        continue;
      }
      if (out.busyOutcome) {
        options.patchView({ lastCommitOutcome: out.busyOutcome });
        return;
      }
      failureBanner = out.banner || m.s7_staging_batch_partial_fail();
      break;
    }
    if (!active()) return;
    await options.onReloadAfterMutation(options.req.signal);
    if (!active()) return;
    options.onRefreshRepositories?.();
    if (failureBanner) options.patchView({ statusBanner: failureBanner });
  });
}
