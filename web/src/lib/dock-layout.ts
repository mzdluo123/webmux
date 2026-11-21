import { z } from "zod";

export type DockDirection = "horizontal" | "vertical";
export type DockEdge = "left" | "right" | "top" | "bottom" | "center";
export type DockRail = "left" | "right" | "bottom";
export type DockGroup = { kind: "group"; ids: string[]; active: string };
export type DockSplit = {
  kind: "split";
  direction: DockDirection;
  first: DockNode;
  second: DockNode;
  ratio: number;
};
export type DockNode = DockGroup | DockSplit;
export type DockRect = { x: number; y: number; width: number; height: number };

export const dockEdges: readonly DockEdge[] = [
  "left",
  "right",
  "top",
  "bottom",
  "center",
];

export function dockGroup(...ids: string[]): DockGroup {
  return { kind: "group", ids, active: ids[0] ?? "" };
}

export function dockSplit(
  direction: DockDirection,
  first: DockNode,
  second: DockNode,
  ratio = 0.5,
): DockSplit {
  return { kind: "split", direction, first, second, ratio };
}

export function dockIds(node: DockNode): string[] {
  return node.kind === "group"
    ? node.ids
    : [...dockIds(node.first), ...dockIds(node.second)];
}

function updateGroups(
  node: DockNode,
  update: (group: DockGroup) => DockNode,
): DockNode {
  if (node.kind === "group") return update(node);
  const first: DockNode = updateGroups(node.first, update);
  const second: DockNode = updateGroups(node.second, update);
  if (first.kind === "group" && !first.ids.length) return second;
  if (second.kind === "group" && !second.ids.length) return first;
  return first === node.first && second === node.second
    ? node
    : { ...node, first, second };
}

export function dockRemove(node: DockNode, id: string): DockNode {
  return updateGroups(node, (group) => {
    if (!group.ids.includes(id)) return group;
    const ids = group.ids.filter((key) => key !== id);
    return {
      ...group,
      ids,
      active: group.active === id ? (ids[0] ?? "") : group.active,
    };
  });
}

export function dockActivate(node: DockNode, id: string): DockNode {
  return updateGroups(node, (group) =>
    group.active !== id && group.ids.includes(id)
      ? { ...group, active: id }
      : group,
  );
}

export function dockMove(
  node: DockNode,
  id: string,
  target: string,
  edge: DockEdge,
  activate = true,
): DockNode {
  if (id === target) return node;
  if (!dockIds(node).includes(target)) {
    const ids = dockIds(node);
    if (!ids.length) return dockGroup(id);
    return dockMove(
      node,
      id,
      ids.find((key) => key !== id) ?? id,
      edge,
      activate,
    );
  }
  return updateGroups(dockRemove(node, id), (group) => {
    if (!group.ids.includes(target)) return group;
    if (edge === "center")
      return {
        ...group,
        ids: [...group.ids, id],
        active: activate ? id : group.active,
      };
    const first = edge === "left" || edge === "top";
    return dockSplit(
      edge === "left" || edge === "right" ? "horizontal" : "vertical",
      first ? dockGroup(id) : group,
      first ? group : dockGroup(id),
    );
  });
}

export function dockFill(
  node: DockNode,
  initial: DockNode,
  allowed: ReadonlySet<string>,
): DockNode {
  const current = new Set(dockIds(node));
  if (!current.size) return initial;
  const initialIds = dockIds(initial);
  function fill(defaultNode: DockNode) {
    if (defaultNode.kind === "split") {
      fill(defaultNode.first);
      fill(defaultNode.second);
      return;
    }
    const missing = defaultNode.ids.filter(
      (id) => allowed.has(id) && !current.has(id),
    );
    if (!missing.length) return;
    const companion = defaultNode.ids.find((id) => current.has(id));
    if (companion) {
      node = updateGroups(node, (group) =>
        group.ids.includes(companion)
          ? { ...group, ids: [...group.ids, ...missing] }
          : group,
      );
    } else {
      const target = initialIds.find((id) => current.has(id)) ?? "";
      node = dockMove(node, missing[0]!, target, "right");
      for (let i = 1; i < missing.length; i++)
        node = dockMove(node, missing[i]!, missing[0]!, "center");
      node = dockActivate(node, defaultNode.active);
    }
    for (const id of missing) current.add(id);
  }
  fill(initial);
  return node;
}

export type DockDivider = {
  path: string;
  direction: DockDirection;
  ratio: number;
  rect: DockRect;
  parent: DockRect;
};

const full: DockRect = { x: 0, y: 0, width: 1, height: 1 };

export function dockGeometry(
  node: DockNode,
  available: ReadonlySet<string>,
  hidden: ReadonlySet<string>,
) {
  const groups = new Map<string, { rect: DockRect; ids: string[] }>();
  const dividers: DockDivider[] = [];
  function hasVisible(current: DockNode): boolean {
    return current.kind === "group"
      ? current.ids.some((id) => available.has(id) && !hidden.has(id))
      : hasVisible(current.first) || hasVisible(current.second);
  }
  function visit(current: DockNode, rect: DockRect, path: string) {
    if (current.kind === "group") {
      const ids = current.ids.filter(
        (id) => available.has(id) && !hidden.has(id),
      );
      const active = ids.includes(current.active) ? current.active : ids[0];
      if (active) groups.set(active, { rect, ids });
      return;
    }
    if (!hasVisible(current.first))
      return visit(current.second, rect, `${path}1`);
    if (!hasVisible(current.second))
      return visit(current.first, rect, `${path}0`);
    const { direction, ratio } = current;
    const { x, y, width, height } = rect;
    const cross = direction === "horizontal";
    const size = (cross ? width : height) * ratio;
    const first = cross ? { ...rect, width: size } : { ...rect, height: size };
    const second = cross
      ? { ...rect, x: x + size, width: width - size }
      : { ...rect, y: y + size, height: height - size };
    dividers.push({
      path,
      direction,
      ratio,
      parent: rect,
      rect: cross
        ? { ...rect, x: second.x, width: 0 }
        : { ...rect, y: second.y, height: 0 },
    });
    visit(current.first, first, `${path}0`);
    visit(current.second, second, `${path}1`);
  }
  visit(node, full, "");
  return { groups, dividers };
}

export function dockResize(
  node: DockNode,
  path: string,
  ratio: number,
): DockNode {
  if (node.kind === "group") return node;
  if (!path) return { ...node, ratio: Math.max(0.1, Math.min(0.9, ratio)) };
  const child = path[0] === "0" ? "first" : "second";
  return { ...node, [child]: dockResize(node[child], path.slice(1), ratio) };
}

export function dockDropEdge(x: number, y: number): DockEdge {
  const distances = [x, 1 - x, y, 1 - y];
  const min = Math.min(...distances);
  return min < 0.25 ? dockEdges[distances.indexOf(min)]! : "center";
}

export function dockRail(rect: DockRect | undefined): DockRail | undefined {
  if (!rect) return undefined;
  const left = rect.x < 0.01;
  const right = rect.x + rect.width > 0.99;
  if (left !== right) return left ? "left" : "right";
  if (rect.y > 0.01 && rect.y + rect.height > 0.99) return "bottom";
  return undefined;
}

const groupSchema = z.object({
  kind: z.literal("group"),
  ids: z.array(z.string().min(1)),
  active: z.string(),
});
const treeSchema: z.ZodType<DockNode> = z.lazy(() =>
  z.union([
    groupSchema,
    z.object({
      kind: z.literal("split"),
      direction: z.enum(["horizontal", "vertical"]),
      ratio: z.number().min(0.1).max(0.9),
      first: treeSchema,
      second: treeSchema,
    }),
  ]),
);
const layoutSchema = z.object({
  version: z.literal(1),
  node: treeSchema,
  hidden: z.array(z.string()),
  rails: z
    .record(z.string(), z.enum(["left", "right", "bottom"]))
    .optional()
    .catch(undefined),
});

export function restoreDock(value: string | null | undefined) {
  if (!value) return undefined;
  try {
    const result = layoutSchema.safeParse(JSON.parse(value));
    if (!result.success) return undefined;
    const ids = dockIds(result.data.node);
    if (ids.length !== new Set(ids).size) return undefined;
    return {
      ...(result.data.rails && { rails: result.data.rails }),
      node: result.data.node,
      hidden: result.data.hidden.filter((id) => ids.includes(id)),
    };
  } catch {
    return undefined;
  }
}
