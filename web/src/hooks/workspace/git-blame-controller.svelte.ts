import { errorMessage } from "../../platform/error";
import { gitBlame } from "../../platform/git-v2-api";
import type { GitBlameResult } from "../../contracts/schemas";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitBlameCrumb {
  rev: string;
  path: string;
  label: string;
}

export interface GitBlameControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
}

export function createGitBlameController(deps: GitBlameControllerDeps) {
  let rev = $state("");
  let path = $state("");
  let data = $state<GitBlameResult | null>(null);
  let loading = $state(false);
  let error = $state("");
  let trail = $state<GitBlameCrumb[]>([]);

  const reqRunner = new LatestRequest();
  let disposed = false;

  function dispose() {
    disposed = true;
    reqRunner.dispose();
  }

  function resetTarget() {
    reqRunner.cancel();
    rev = "";
    path = "";
    data = null;
    loading = false;
    error = "";
    trail = [];
  }

  function primeTarget(initialPath: string, initialRev: string) {
    reqRunner.cancel();
    path = initialPath;
    rev = initialRev;
    data = null;
    loading = false;
    error = "";
    trail = [];
  }

  async function load(targetRev = rev, targetPath = path, pushCrumb = false) {
    const repoId = deps.getRepoId();
    if (!repoId || !targetPath || disposed) return;
    if (pushCrumb && data) {
      trail = [
        ...trail,
        {
          rev: data.commitOid,
          path: data.path,
          label: `${data.commitOid.slice(0, 7)} · ${data.path}`,
        },
      ];
    }
    rev = targetRev;
    path = targetPath;
    loading = true;
    error = "";
    const req = reqRunner.begin();
    try {
      const res = await gitBlame(
        repoId,
        {
          path: targetPath,
          rev: targetRev || undefined,
          worktreeId: deps.getWorktreeId(),
        },
        req.signal,
      );
      if (!req.current() || disposed || repoId !== deps.getRepoId()) return;
      data = res;
    } catch (err) {
      if (req.reportable()) error = errorMessage(err);
    } finally {
      if (req.current()) loading = false;
    }
  }

  function stepBack(index: number) {
    const target = trail[index];
    if (!target) return;
    trail = trail.slice(0, index);
    void load(target.rev, target.path, false);
  }

  return {
    get rev() {
      return rev;
    },
    get path() {
      return path;
    },
    get data() {
      return data;
    },
    get loading() {
      return loading;
    },
    get error() {
      return error;
    },
    get trail() {
      return trail;
    },
    dispose,
    resetTarget,
    primeTarget,
    load,
    stepBack,
  };
}
