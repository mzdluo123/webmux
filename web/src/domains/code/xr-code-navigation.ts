import type { CodeListing } from "../../contracts/schemas";
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
        label: `加载更多 ${path || "根目录"}`,
        run: () => options.more(path),
      });
  }
  visit(options.listing, "");
  for (const [path, message] of options.errors)
    lines.push({ text: `${path}: ${message}`, tone: "error" as const });
  if (!actions.length)
    lines.push({ text: "当前目录没有可打开的文件。", tone: "muted" as const });
  return {
    id: "code-navigation",
    kind: "code",
    title: "文件导航",
    lines,
    actions,
  };
}
