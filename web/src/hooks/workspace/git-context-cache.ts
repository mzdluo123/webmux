import { APIError } from "../../platform/api";
import { gitContext } from "../../platform/git-api";

export function createGitContextRunner() {
  let cached: {
    repoId: string;
    worktreeId: string | undefined;
    contextId: string;
  } | null = null;

  function reset() {
    cached = null;
  }

  function seed(
    repoId: string,
    worktreeId: string | undefined,
    contextId: string,
  ) {
    if (!repoId || !contextId) return;
    cached = { repoId, worktreeId, contextId };
  }

  async function ensure(
    repoId: string,
    worktreeId: string | undefined,
    signal: AbortSignal,
    force = false,
  ): Promise<string> {
    if (
      !force &&
      cached &&
      cached.repoId === repoId &&
      cached.worktreeId === worktreeId &&
      cached.contextId
    ) {
      return cached.contextId;
    }
    const ctx = await gitContext(repoId, worktreeId, signal);
    cached = { repoId, worktreeId, contextId: ctx.contextId };
    return ctx.contextId;
  }

  async function withContext<T>(
    repoId: string,
    worktreeId: string | undefined,
    signal: AbortSignal,
    run: (contextId: string) => Promise<T>,
  ): Promise<T> {
    const first = await ensure(repoId, worktreeId, signal, false);
    try {
      return await run(first);
    } catch (err) {
      if (
        !signal.aborted &&
        err instanceof APIError &&
        (err.status === 409 || err.status === 410)
      ) {
        const next = await ensure(repoId, worktreeId, signal, true);
        return await run(next);
      }
      throw err;
    }
  }

  return { reset, seed, ensure, withContext };
}
