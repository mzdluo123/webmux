import type { ReportedCwd, SessionInfo } from "./schemas";

export interface LocationHints {
  launch: string | null;
  reported: ReportedCwd | null;
}

// Shell reports are navigation hints. Grouping and ownership stay on the
// immutable launch directory and workspace association.
export function locationHints(
  session: Pick<SessionInfo, "cwd" | "reportedCwd">,
  groupPath?: string,
): LocationHints {
  const reported = session.reportedCwd;
  return {
    launch:
      groupPath !== undefined && session.cwd === groupPath ? null : session.cwd,
    reported:
      reported && (reported.remote || reported.path !== session.cwd)
        ? reported
        : null,
  };
}

export function matchesSessionQuery(
  session: SessionInfo,
  search: string,
  repositoryName?: string,
  branch?: string | null,
) {
  return [
    session.title,
    session.cwd,
    session.reportedCwd?.path,
    session.reportedCwd?.host,
    repositoryName,
    branch,
  ].some((value) => value?.toLocaleLowerCase().includes(search));
}

export type PathResolution =
  | { ok: true; path: string; relative: string }
  | { ok: false; reason: "empty" | "invalid" | "outside" | "root" };

// oxlint-disable-next-line no-control-regex -- Reject control bytes before path resolution.
const controlCharacters = /[\x00-\x1f\x7f-\x9f]/;

function normalizeAbsolute(value: string) {
  if (!value.startsWith("/") || controlCharacters.test(value)) return null;
  const parts: string[] = [];
  for (const part of value.split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") {
      if (!parts.length) return null;
      parts.pop();
      continue;
    }
    parts.push(part);
  }
  return `/${parts.join("/")}`;
}

export function resolveSessionPath(
  base: string,
  input: string,
  root: string,
): PathResolution {
  const value = input.trim();
  if (!value) return { ok: false, reason: "empty" };
  const rootPath = normalizeAbsolute(root);
  const basePath = normalizeAbsolute(base);
  if (!rootPath || !basePath) return { ok: false, reason: "invalid" };
  const absolute = normalizeAbsolute(
    value.startsWith("/") ? value : `${basePath}/${value}`,
  );
  if (!absolute) return { ok: false, reason: "invalid" };
  const prefix = rootPath === "/" ? "/" : `${rootPath}/`;
  if (absolute !== rootPath && !absolute.startsWith(prefix)) {
    return { ok: false, reason: "outside" };
  }
  if (absolute === rootPath) return { ok: false, reason: "root" };
  return {
    ok: true,
    path: absolute,
    relative: absolute.slice(rootPath === "/" ? 1 : rootPath.length + 1),
  };
}
