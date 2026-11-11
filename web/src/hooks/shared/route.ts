// Route matching for the single-page app. `matchRoute` looks at `pathname`,
// while `parseGitRouteQuery` and `buildGitRouteHref` handle `/git` search state.
import type {
  GitHistoryScope,
  GitWorkbenchMode,
} from "../../contracts/git-workbench-schemas";

export interface GitRouteQuery {
  view?: GitWorkbenchMode;
  scope?: GitHistoryScope;
  branch?: string;
  commit?: string;
  q?: string;
  returnSession?: string;
}

export interface GitRouteTarget extends GitRouteQuery {
  repositoryId: string;
  worktreeId?: string;
}

export type Route =
  | { name: "terminal"; id: string }
  | { name: "code"; id: string }
  | { name: "handoff"; sourceId: string }
  | { name: "git"; repositoryId: string; worktreeId?: string }
  | { name: "settings" }
  | { name: "settings-tailscale" }
  | { name: "settings-tunnel" }
  // Exchanges a ticket for the port-forward gateway cookie, then leaves.
  | { name: "forward-auth" }
  // Chat mode's home: write a task and start an agent in a new worktree.
  | { name: "quick-start" }
  | { name: "sessions" };

const sessionIdPattern = /^[a-z2-7]{8}$/;
const gitIdPattern = /^[A-Za-z0-9_-]{1,64}$/;
const gitCommitPattern = /^[0-9a-f]{4,64}$/i;

// Unknown paths fall back to the session list; there is no 404 page.
export function matchRoute(pathname: string): Route {
  const session = /^\/s\/([a-z2-7]{8})$/.exec(pathname);
  if (session?.[1]) return { name: "terminal", id: session[1] };
  const handoff = /^\/handoff\/([a-f0-9]{32})$/.exec(pathname);
  if (handoff?.[1]) return { name: "handoff", sourceId: handoff[1] };
  const code = /^\/code\/([a-z2-7]{8})$/.exec(pathname);
  if (code?.[1]) return { name: "code", id: code[1] };
  if (pathname === "/git") return { name: "git", repositoryId: "" };
  const gitWorktree =
    /^\/git\/([A-Za-z0-9_-]{1,64})\/worktrees\/([A-Za-z0-9_-]{1,64})$/.exec(
      pathname,
    );
  if (gitWorktree?.[1] && gitWorktree[2]) {
    return {
      name: "git",
      repositoryId: gitWorktree[1],
      worktreeId: gitWorktree[2],
    };
  }
  const gitRepo = /^\/git\/([A-Za-z0-9_-]{1,64})$/.exec(pathname);
  if (gitRepo?.[1] && gitIdPattern.test(gitRepo[1])) {
    return { name: "git", repositoryId: gitRepo[1] };
  }
  if (pathname === "/settings/tailscale") return { name: "settings-tailscale" };
  if (pathname === "/settings/tunnel") return { name: "settings-tunnel" };
  if (pathname === "/forward-auth") return { name: "forward-auth" };
  if (pathname === "/new") return { name: "quick-start" };
  if (pathname === "/settings") return { name: "settings" };
  return { name: "sessions" };
}

export function parseGitRouteQuery(search: string): GitRouteQuery {
  const params = new URLSearchParams(search);
  const out: GitRouteQuery = {};

  const rawView = params.get("view");
  if (rawView === "log" || rawView === "commit" || rawView === "branches") {
    out.view = rawView;
  }

  const rawScope = params.get("scope");
  if (rawScope === "task") {
    out.scope = "branch";
  } else if (
    rawScope === "all" ||
    rawScope === "branch" ||
    rawScope === "to-push" ||
    rawScope === "incoming"
  ) {
    out.scope = rawScope;
  }

  const rawBranch = params.get("branch")?.trim();
  if (rawBranch && rawBranch.length <= 256 && !/[\r\n\0]/.test(rawBranch)) {
    out.branch = rawBranch;
  }

  const rawCommit = params.get("commit")?.trim();
  if (rawCommit && gitCommitPattern.test(rawCommit)) {
    out.commit = rawCommit;
  }

  if (params.has("q")) {
    const rawQ = params.get("q") ?? "";
    if (rawQ.length <= 512) out.q = rawQ;
  }

  const rawReturnSession = params.get("returnSession")?.trim();
  if (rawReturnSession && sessionIdPattern.test(rawReturnSession)) {
    out.returnSession = rawReturnSession;
  }

  return out;
}

export function buildGitRouteHref(target: GitRouteTarget): string {
  const repoId = target.repositoryId.trim();
  const wtId = target.worktreeId?.trim() ?? "";
  let base = "/git";
  if (repoId) {
    base = wtId
      ? `/git/${encodeURIComponent(repoId)}/worktrees/${encodeURIComponent(wtId)}`
      : `/git/${encodeURIComponent(repoId)}`;
  }

  const params = new URLSearchParams();
  if (target.view) params.set("view", target.view);
  if (target.scope && target.scope !== "all") params.set("scope", target.scope);
  if (target.branch) params.set("branch", target.branch);
  if (target.commit) params.set("commit", target.commit);
  if (target.q) params.set("q", target.q);
  if (target.returnSession && sessionIdPattern.test(target.returnSession)) {
    params.set("returnSession", target.returnSession);
  }

  const encoded = params.toString();
  return encoded ? `${base}?${encoded}` : base;
}

// The gateway address `/forward-auth?next=` returns to, or null. Like the
// server, it admits only paths under /forward/, so the page never sends a
// ticket toward another origin.
export function forwardAuthNext(search: string): string | null {
  const next = new URLSearchParams(search).get("next");
  if (!next?.startsWith("/forward/") || /\/\/|[\\\r\n]/.test(next)) return null;
  return next;
}
