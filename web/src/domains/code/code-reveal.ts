import type { z } from "zod";
import { m } from "$lib/paraglide/messages.js";
import { api, apiBlob, APIError } from "../../platform/api";
import { codeRawPath, isImageType } from "./code-image";
import {
  aborted,
  type CodeContext,
  type CodeTabOrigin,
  type CodeWorkspaceTab,
} from "./code-workspace";
import {
  SymlinkTargetError,
  targetFailure,
  type ChatCodeLink,
  type MarkdownWorkspace,
} from "./markdown-link";
import {
  CodeChangesSchema,
  CodeContextSchema,
  CodeFileSchema,
  CodeLocationSchema,
  type CodeChange,
  type CodeReveal,
} from "../../contracts/schemas";

// Resolves a root-relative path from a Markdown preview. The server walks
// the absolute path through symlinks, so a canonical path that differs from
// the requested one went through a link.
async function resolveWorkspaceTarget(
  base: string,
  context: CodeContext | undefined,
  params: (extra?: Record<string, string>) => URLSearchParams,
  path: string,
  signal?: AbortSignal,
) {
  if (!context) throw new Error(m.s5_code_workspace_not_ready());
  const root = context.rootPath.replace(/\/+$/, "");
  const target = await api(
    `${base}/locate?${params({ path: path ? `${root}/${path}` : root || "/" })}`,
    CodeLocationSchema,
    "GET",
    undefined,
    signal,
  );
  if (target.path !== path) throw new SymlinkTargetError();
  return target;
}

export function createMarkdownWorkspace(options: {
  id: () => string;
  base: () => string;
  getContext: () => CodeContext | undefined;
  params: (extra?: Record<string, string>) => URLSearchParams;
  show: (target: z.infer<typeof CodeLocationSchema>) => Promise<string>;
  openFileInOrigin: (
    origin: CodeTabOrigin,
    path: string,
    opaqueId: string,
  ) => Promise<string>;
  getTabs: () => CodeWorkspaceTab[];
  nextHeadingKey: () => number;
  externalOrigin?: boolean;
}): MarkdownWorkspace {
  const applyHeading = (contextId: string, nodeId: string, heading: string) => {
    if (!heading) return;
    const tab = options
      .getTabs()
      .find((t) => t.key === `${contextId}:${nodeId}:worktree`);
    if (tab) tab.heading = { slug: heading, key: options.nextHeadingKey() };
  };
  const openAbsolute = async (absolutePath: string, heading: string) => {
    const context = options.getContext();
    let targetOrigin: CodeTabOrigin | undefined;
    let target: z.infer<typeof CodeLocationSchema> | undefined;
    let firstError: unknown;
    if (context && context.rootKind !== "filesystem") {
      try {
        target = await api(
          `${options.base()}/locate?${options.params({ contextId: context.contextId, path: absolutePath })}`,
          CodeLocationSchema,
        );
        targetOrigin = context;
      } catch (e) {
        firstError = e;
      }
    }
    if (!target) {
      if (
        !firstError ||
        (firstError instanceof APIError && firstError.status === 403)
      ) {
        try {
          const outside = await locateOutsideRoot(
            options.base(),
            options.params,
            [{ path: absolutePath, raw: false }],
            context?.rootKind === "artifacts",
          );
          if (outside) {
            targetOrigin = outside.origin;
            target = outside.location;
          }
        } catch (e) {
          firstError = e;
        }
      }
    }
    if (!target || !targetOrigin) {
      return targetFailure("link", firstError ?? new APIError(403, ""));
    }
    const failure =
      targetOrigin.contextId === context?.contextId
        ? await options.show(target)
        : target.kind === "file"
          ? await options.openFileInOrigin(
              targetOrigin,
              target.path,
              target.nodeId,
            )
          : m.s5_link_outside_workspace();
    if (failure) return m.s5_link_target_unreadable({ failure });
    applyHeading(targetOrigin.contextId, target.nodeId, heading);
    return "";
  };
  return {
    async open(path, heading = "") {
      if (path.startsWith("/") || options.externalOrigin) {
        return openAbsolute(path.startsWith("/") ? path : `/${path}`, heading);
      }
      const context = options.getContext();
      let target: z.infer<typeof CodeLocationSchema>;
      try {
        target = await resolveWorkspaceTarget(
          options.base(),
          context,
          options.params,
          path,
        );
      } catch (e) {
        if (e instanceof APIError && e.status === 403 && context) {
          const root = context.rootPath.replace(/\/+$/, "");
          return openAbsolute(path ? `${root}/${path}` : root || "/", heading);
        }
        return targetFailure("link", e);
      }
      const failure = await options.show(target);
      if (failure) return m.s5_link_target_unreadable({ failure });
      if (context) applyHeading(context.contextId, target.nodeId, heading);
      return "";
    },
    async image(path, signal) {
      try {
        const target = await resolveWorkspaceTarget(
          options.base(),
          options.getContext(),
          options.params,
          path,
          signal,
        );
        if (target.kind !== "file")
          return { ok: false, reason: m.s5_not_previewable_image() };
        const { snapshotId, document: doc } = await api(
          `${options.base()}/file?${options.params({ nodeId: target.nodeId })}`,
          CodeFileSchema,
          "GET",
          undefined,
          signal,
        );
        if (doc.kind === "too-large" && isImageType(doc.mediaType))
          return { ok: false, reason: m.s5_exceeds_20mib_preview_limit() };
        if (
          doc.kind !== "image" &&
          !(doc.kind === "text" && doc.mediaType === "image/svg+xml")
        )
          return { ok: false, reason: m.s5_not_previewable_image() };
        const blob = await apiBlob(
          codeRawPath(options.id(), snapshotId),
          signal,
        );
        // An <img> renders SVG only with its media type; slicing is a view.
        return {
          ok: true,
          path: doc.path,
          blob:
            blob.type === doc.mediaType
              ? blob
              : blob.slice(0, blob.size, doc.mediaType),
        };
      } catch (e) {
        if (aborted(e)) throw e;
        return {
          ok: false,
          reason: targetFailure("image", e),
          retry: e instanceof APIError && e.status === 429,
        };
      }
    },
  };
}

// Candidate order: a relative path resolves against the session's current
// directory first (where the agent runs, so where its paths are written
// from), then the repository root. A same-named file under the current
// directory therefore wins over the repository-root one.
function revealCandidates(origin: CodeContext, target: CodeReveal) {
  const rootPrefix = origin.rootPath.replace(/\/+$/, "");
  const list: { path: string; raw?: boolean }[] = [{ path: target.path }];
  const add = (path: string, raw = false) => {
    if (path && !list.some((item) => item.path === path))
      list.push({ path, raw });
  };
  if (!target.fromUri) {
    if (!target.path.startsWith("/")) add(`${rootPrefix}/${target.path}`);
    else if (
      target.path !== origin.rootPath &&
      !target.path.startsWith(`${rootPrefix}/`)
    )
      add(`${rootPrefix}${target.path}`);
  }
  if (target.rawPath && target.rawPath !== target.path) {
    add(target.rawPath, true);
    if (!target.fromUri && !target.rawPath.startsWith("/"))
      add(`${rootPrefix}/${target.rawPath}`, true);
  }
  return list;
}

async function locateOutsideRoot(
  base: string,
  params: (extra?: Record<string, string>) => URLSearchParams,
  candidates: { path: string; raw: boolean }[],
  skipArtifacts = false,
) {
  if (!skipArtifacts && candidates.some((item) => item.path.startsWith("/"))) {
    try {
      const artifactsOrigin = await api(
        `${base}/context?root=artifacts`,
        CodeContextSchema,
      );
      for (const item of candidates) {
        try {
          const location = await api(
            `${base}/locate?${params({ contextId: artifactsOrigin.contextId, path: item.path })}`,
            CodeLocationSchema,
          );
          return {
            origin: artifactsOrigin,
            location,
            usedRawPath: item.raw,
          };
        } catch {
          // Try rawPath if present; otherwise fall through to filesystem root.
        }
      }
    } catch {
      // No existing artifacts folder; fall through to filesystem root.
    }
  }
  let fsOrigin: CodeContext;
  try {
    fsOrigin = await api(`${base}/context?root=filesystem`, CodeContextSchema);
  } catch {
    return undefined;
  }
  let fsError: unknown;
  for (const item of candidates) {
    try {
      const location = await api(
        `${base}/locate?${params({ contextId: fsOrigin.contextId, path: item.path })}`,
        CodeLocationSchema,
      );
      if (location.kind === "file") {
        return {
          origin: fsOrigin,
          location,
          usedRawPath: item.raw,
        };
      }
    } catch (e) {
      if (
        !fsError &&
        e instanceof APIError &&
        e.status !== 404 &&
        e.status !== 403
      )
        fsError = e;
    }
  }
  if (fsError) throw fsError;
  return undefined;
}

export async function locatePathTarget(
  base: string,
  params: (extra?: Record<string, string>) => URLSearchParams,
  origin: CodeContext,
  path: string,
) {
  try {
    const location = await api(
      `${base}/locate?${params({ path })}`,
      CodeLocationSchema,
    );
    return { origin, location };
  } catch (e) {
    if (!(e instanceof APIError && e.status === 403)) throw e;
    const fallback = await locateOutsideRoot(
      base,
      params,
      [{ path, raw: false }],
      origin.rootKind === "artifacts",
    );
    if (!fallback) throw e;
    return { origin: fallback.origin, location: fallback.location };
  }
}

async function locateRevealTarget(
  base: string,
  params: (extra?: Record<string, string>) => URLSearchParams,
  origin: CodeContext,
  target: CodeReveal,
) {
  const candidates = revealCandidates(origin, target);
  let firstError: unknown;
  for (let i = 0; i < candidates.length; i++) {
    const candidate = candidates[i]!;
    try {
      const location = await api(
        `${base}/locate?${params({ contextId: origin.contextId, path: candidate.path })}`,
        CodeLocationSchema,
      );
      return { origin, location, usedRawPath: !!candidate.raw };
    } catch (e) {
      if (i === 0) firstError = e;
      if (
        !(
          e instanceof APIError &&
          (e.status === 404 ||
            e.status === 409 ||
            (!target.fromUri && e.status === 403))
        )
      )
        break;
    }
  }
  if (firstError instanceof APIError && firstError.status === 403) {
    const fallbackPaths =
      target.rawPath && target.rawPath !== target.path
        ? [
            { path: target.path, raw: false },
            { path: target.rawPath, raw: true },
          ]
        : [{ path: target.path, raw: false }];
    const outside = await locateOutsideRoot(base, params, fallbackPaths);
    if (outside) return outside;
  }
  throw firstError;
}

export type RevealResolution =
  | {
      kind: "directory";
      origin: CodeContext;
      location: z.infer<typeof CodeLocationSchema>;
    }
  | { kind: "diff"; origin: CodeContext; change: CodeChange }
  | {
      kind: "file";
      origin: CodeContext;
      location: z.infer<typeof CodeLocationSchema>;
      mode: "file" | "diff";
      usedRawPath: boolean;
    };

export async function resolveRevealAction(
  base: string,
  params: (extra?: Record<string, string>) => URLSearchParams,
  origin: CodeContext,
  target: CodeReveal,
): Promise<RevealResolution> {
  const {
    origin: resolvedOrigin,
    location,
    usedRawPath,
  } = await locateRevealTarget(base, params, origin, target);
  if (location.kind !== "file") {
    return { kind: "directory", origin: resolvedOrigin, location };
  }
  const mode =
    target.mode ??
    (target.line !== undefined || target.heading ? "file" : "diff");
  if (mode === "diff" && resolvedOrigin.diffAvailable) {
    const found = await api(
      `${base}/changes?${params({ contextId: resolvedOrigin.contextId, nodeId: location.nodeId })}`,
      CodeChangesSchema,
    );
    const change =
      found.entries.find((entry) => entry.group !== "staged") ??
      found.entries[0];
    if (change) return { kind: "diff", origin: resolvedOrigin, change };
  }
  return { kind: "file", origin: resolvedOrigin, location, mode, usedRawPath };
}

/**
 * Resolves a chat code-location link against the browsable roots without
 * opening anything, returning a failure reason to show in place or "" when
 * the target resolves. A context request that itself fails resolves to ""
 * so the code panel keeps reporting that failure instead.
 */
export async function checkRevealTarget(
  base: string,
  target: ChatCodeLink,
): Promise<string> {
  let origin: CodeContext;
  try {
    origin = await api(`${base}/context`, CodeContextSchema);
  } catch {
    return "";
  }
  const params = (extra: Record<string, string> = {}) =>
    new URLSearchParams({ contextId: origin.contextId, ...extra });
  try {
    await locateRevealTarget(base, params, origin, {
      ...target,
      mode: "file",
      key: 0,
    });
    return "";
  } catch (e) {
    return targetFailure("link", e);
  }
}
