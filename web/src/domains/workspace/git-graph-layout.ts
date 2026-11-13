import type {
  GitHistoryEdge,
  GitHistoryRow,
} from "../../contracts/git-workbench-schemas";

export const GIT_GRAPH_LANE_WIDTH = 14;
export const GIT_GRAPH_ROW_HEIGHT = 28;
export const GIT_GRAPH_LEFT_PAD = 8;

export type GitLaneTone = 0 | 1 | 2 | 3 | 4 | 5;

interface GitGraphRail {
  lane: number;
  x: number;
  y1: number;
  y2: number;
  tone: GitLaneTone;
}

interface GitGraphPathSegment {
  key: string;
  d: string;
  kind: GitHistoryEdge["kind"];
  dashed: boolean;
  isBoundary: boolean;
  targetOid: string;
  tone: GitLaneTone;
}

export interface GitGraphRowLayout {
  oid: string;
  rowIndex: number;
  width: number;
  height: number;
  nodeX: number;
  nodeY: number;
  isMerge: boolean;
  directParentsMissing: boolean;
  nodeTone: GitLaneTone;
  rails: GitGraphRail[];
  segments: GitGraphPathSegment[];
}

export function laneToX(lane: number): number {
  return GIT_GRAPH_LEFT_PAD + lane * GIT_GRAPH_LANE_WIDTH;
}

export function laneTone(lane: number): GitLaneTone {
  return (Math.abs(lane) % 6) as GitLaneTone;
}

export function laneStrokeClass(tone: GitLaneTone): string {
  switch (tone) {
    case 0:
      return "stroke-git-primary";
    case 1:
      return "stroke-success";
    case 2:
      return "stroke-warning";
    case 3:
      return "stroke-info";
    case 4:
      return "stroke-destructive";
    default:
      return "stroke-muted-foreground";
  }
}

export function laneFillClass(tone: GitLaneTone): string {
  switch (tone) {
    case 0:
      return "fill-git-primary";
    case 1:
      return "fill-success";
    case 2:
      return "fill-warning";
    case 3:
      return "fill-info";
    case 4:
      return "fill-destructive";
    default:
      return "fill-muted-foreground";
  }
}

/**
 * Computes per-row SVG coordinates for multi-lane commit topology:
 * - `direct` edges render solid curves/lines to in-page parents;
 * - `indirect` edges (intermediate ancestors omitted by path/keyword filters)
 *   render dashed curves and preserve `directParentsMissing` metadata;
 * - `boundary` edges render downward continuation stubs at the bottom of the page.
 */
export function computeGitGraphLayout(
  rows: readonly GitHistoryRow[],
): GitGraphRowLayout[] {
  if (rows.length === 0) {
    return [];
  }

  const rowIndexByOid = new Map<string, number>();
  let maxLane = 0;
  for (let i = 0; i < rows.length; i++) {
    const r = rows[i]!;
    rowIndexByOid.set(r.oid, i);
    if (r.lane > maxLane) maxLane = r.lane;
    for (const active of r.activeLanes) {
      if (active > maxLane) maxLane = active;
    }
    for (const edge of r.edges) {
      if (edge.fromLane > maxLane) maxLane = edge.fromLane;
      if (edge.toLane > maxLane) maxLane = edge.toLane;
    }
  }

  const totalWidth =
    GIT_GRAPH_LEFT_PAD * 2 + Math.max(1, maxLane + 1) * GIT_GRAPH_LANE_WIDTH;
  const midY = GIT_GRAPH_ROW_HEIGHT / 2;

  return rows.map((row, index) => {
    const prevActive = index > 0 ? (rows[index - 1]?.activeLanes ?? []) : [];
    const rails: GitGraphRail[] = [];

    for (const lane of row.activeLanes) {
      if (lane === row.lane) {
        if (prevActive.includes(lane)) {
          rails.push({
            lane,
            x: laneToX(lane),
            y1: 0,
            y2: midY,
            tone: laneTone(lane),
          });
        }
        continue;
      }
      // Pass-through lane rail across this row.
      const continuesFromPrev = prevActive.includes(lane);
      rails.push({
        lane,
        x: laneToX(lane),
        y1: continuesFromPrev ? 0 : midY,
        y2: GIT_GRAPH_ROW_HEIGHT,
        tone: laneTone(lane),
      });
    }

    const segments: GitGraphPathSegment[] = row.edges.map((edge, edgeIdx) => {
      const startX = laneToX(edge.fromLane);
      const endX = laneToX(edge.toLane);
      const targetRow = rowIndexByOid.get(edge.targetOid);
      const isBoundary = edge.kind === "boundary" || targetRow === undefined;
      const dashed = edge.kind === "indirect";
      const endY = GIT_GRAPH_ROW_HEIGHT;

      let d: string;
      if (startX === endX) {
        d = `M ${startX} ${midY} L ${endX} ${endY}`;
      } else {
        const ctrlY = midY + (endY - midY) * 0.55;
        d = `M ${startX} ${midY} C ${startX} ${ctrlY}, ${endX} ${ctrlY}, ${endX} ${endY}`;
      }

      return {
        key: `${row.oid}-${edge.targetOid}-${edgeIdx}`,
        d,
        kind: isBoundary && edge.kind === "direct" ? "boundary" : edge.kind,
        dashed,
        isBoundary,
        targetOid: edge.targetOid,
        tone: laneTone(edge.toLane),
      };
    });

    return {
      oid: row.oid,
      rowIndex: index,
      width: totalWidth,
      height: GIT_GRAPH_ROW_HEIGHT,
      nodeX: laneToX(row.lane),
      nodeY: midY,
      isMerge: row.parents.length > 1,
      directParentsMissing: row.directParentsMissing,
      nodeTone: laneTone(row.lane),
      rails,
      segments,
    };
  });
}
