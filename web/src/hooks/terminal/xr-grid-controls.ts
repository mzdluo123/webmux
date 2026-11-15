import type { XrRegion } from "./xr-surface";

/** Grid editing is an explicit confirmation, never a panel resize effect. */
export function xrGridControls(
  grid: [number, number],
  finish: (grid: [number, number] | undefined) => void,
  redraw: () => void,
): XrRegion[] {
  return ["列 −", "列 +", "行 −", "行 +", "确认网格", "取消"].map(
    (label, index) => ({
      id: `grid:${index}`,
      label,
      x: 12 + index * 166,
      y: 116,
      width: 162,
      height: 52,
      run: () => {
        if (index >= 4) {
          finish(index === 4 ? grid : undefined);
          return;
        }
        const axis = index < 2 ? 0 : 1;
        grid[axis] = Math.max(
          axis === 0 ? 20 : 5,
          Math.min(axis === 0 ? 500 : 200, grid[axis] + (index % 2 ? 1 : -1)),
        );
        redraw();
      },
    }),
  );
}
