import type { XrPanelLayout } from "../../domains/terminal/xr-layout";
import type {
  XrAction,
  XrSessionSource,
  XrTextLine,
} from "../../domains/terminal/xr-workspace";
import { xrGraphemes } from "../../domains/terminal/xr-text";
import type { XrRegion, XrSurface } from "./xr-surface";

export function drawXrPanelContent(
  surface: XrSurface,
  source: XrSessionSource,
  layout: XrPanelLayout,
  state: {
    pending?: XrAction;
    confirmationOffset: number;
    lines: readonly XrTextLine[];
    column: number;
    notice: string;
  },
) {
  if (state.pending) {
    surface.text("确认操作", 40, 140, "#edf4ef", "30px system-ui");
    const message = xrGraphemes(state.pending.confirm ?? state.pending.label);
    for (let row = 0; row < 6; row++) {
      const start = (state.confirmationOffset + row) * 36;
      surface.text(
        message.slice(start, start + 36).join(""),
        40,
        210 + row * 32,
      );
    }
  } else {
    if (layout.kind === "terminal" && !source.terminal)
      surface.text("正在挂载现有终端…", 32, 140);
    state.lines.forEach((line, index) =>
      surface.text(
        xrGraphemes(line.text.replaceAll("\t", "    "))
          .slice(state.column, state.column + 40)
          .join(""),
        30,
        120 + index * 28,
        line.tone === "added"
          ? "#83d3a0"
          : line.tone === "removed" || line.tone === "error"
            ? "#f0a0a0"
            : line.tone === "muted"
              ? "#a1ada5"
              : "#edf4ef",
      ),
    );
  }
  if (!source.graphicsUnsupported)
    surface.text(
      state.notice ||
        (source.canInput(layout.kind, layout.sourceId)
          ? "标题拖动 · 内容滚动不改变输入目标"
          : "只读：当前来源不允许输入 · 可通过会话操作返回 2D"),
      20,
      724,
      "#a1ada5",
      "20px system-ui",
    );
}

export function drawUnavailableXrPanel(
  surface: XrSurface,
  layout: XrPanelLayout,
  close: XrRegion,
) {
  surface.regions = [
    {
      id: "move",
      label: `${layout.sessionId} · 视图 ${layout.id.slice(-8)} · 暂不可用`,
      x: 8,
      y: 4,
      width: 800,
      height: 58,
      mode: "move",
    },
    close,
  ];
  surface.draw(() => {
    surface.text("来源暂不可用 / 正在连接；已保留绑定和几何。", 30, 140);
    surface.text("当前视图只读，恢复前不会接收键盘输入。", 30, 180);
  });
}
