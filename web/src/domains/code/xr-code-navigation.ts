import * as m from "$lib/paraglide/messages.js";
import type { CodeChange, CodeListing } from "../../contracts/schemas";
import type { XrAction, XrTextPanel } from "../terminal/xr-workspace";

export function xrTreePanel(options: {
  listing: CodeListing | undefined;
  expanded: ReadonlySet<string>;
  branches: ReadonlyMap<string, CodeListing>;
  loading: ReadonlySet<string>;
  errors: ReadonlyMap<string, string>;
  toggle(path: string, open: boolean): void;
  open(path: string, id: string): void;
  more(path: string): void;
}): XrTextPanel {
  const actions: XrAction[] = [];
  const lines = [];
  function visit(listing: CodeListing | undefined, path: string) {
    if (!listing) return;
    for (const node of listing.children) {
      const directory = node.kind === "directory";
      const open = options.expanded.has(node.path);
      actions.push({
        id: node.nodeId,
        label: `${directory ? (open ? "▾ " : "▸ ") : ""}${node.path}`,
        disabled:
          options.loading.has(node.path) ||
          (!directory && node.kind !== "file"),
        run: () =>
          directory
            ? options.toggle(node.path, !open)
            : options.open(node.path, node.nodeId),
      });
      if (directory && open) visit(options.branches.get(node.path), node.path);
    }
    if (listing.nextCursor)
      actions.push({
        id: `more:${path}`,
        label: m.s5_xr_load_more_dir({ path: path || m.s5_root_directory() }),
        run: () => options.more(path),
      });
  }
  visit(options.listing, "");
  for (const [path, message] of options.errors)
    lines.push({ text: `${path}: ${message}`, tone: "error" as const });
  if (!actions.length)
    lines.push({ text: m.s5_xr_no_openable_files(), tone: "muted" as const });
  return {
    id: "code-navigation",
    kind: "code",
    title: m.s5_file_navigation(),
    lines,
    actions,
  };
}

export function xrNavigatorPanel(options: {
  nav: string;
  treePanel?: XrTextPanel;
  pending: boolean;
  navStatus: string;
  changesError: string;
  changesEntries?: readonly CodeChange[];
  changesNextCursor?: string;
  changesEnabled: boolean;
  openDiff: (change: CodeChange) => void;
  switchMode: () => void | Promise<void>;
  loadMoreChanges: () => void | Promise<void>;
}): XrTextPanel {
  const panel: XrTextPanel =
    options.nav === "files"
      ? (options.treePanel ?? {
          id: "code-navigation",
          kind: "code",
          title: m.s5_file_navigation(),
          lines: [
            {
              text: options.pending
                ? m.s5_reading_dir_full()
                : options.navStatus || m.s5_dir_unavailable(),
            },
          ],
        })
      : {
          id: "code-navigation",
          kind: "code",
          title: m.s5_git_changes(),
          lines: options.changesError
            ? [{ text: options.changesError, tone: "error" }]
            : [],
          actions: options.changesEntries?.map((change) => ({
            id: change.changeId,
            label: `${change.status} ${change.newPath || change.oldPath}`,
            run: () => options.openDiff(change),
          })),
        };
  return {
    ...panel,
    actions: [
      {
        id: "navigation-mode",
        label:
          options.nav === "files"
            ? m.s5_view_git_changes()
            : m.s5_browse_files(),
        disabled: options.nav === "files" && !options.changesEnabled,
        run: options.switchMode,
      },
      ...(panel.actions ?? []),
      ...(options.nav === "changes" && options.changesNextCursor
        ? [
            {
              id: "more-changes",
              label: m.s5_load_more_changes(),
              run: options.loadMoreChanges,
            },
          ]
        : []),
    ],
  };
}
