import { m } from "$lib/paraglide/messages.js";
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
    surface.text(
      m.s4_xr_confirm_action_heading(),
      40,
      140,
      "#edf4ef",
      "30px system-ui",
    );
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
      surface.text(m.s4_xr_mounting_terminal(), 32, 140);
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
          ? m.s4_xr_panel_hint_draggable()
          : m.s4_xr_panel_hint_readonly()),
      20,
      724,
      "#a1ada5",
      "20px system-ui",
    );
}

export function confirmXrPanelRegions(
  pending: XrAction,
  offset: number,
  setOffset: (next: number) => void,
  confirm: () => void,
  cancel: () => void,
): XrRegion[] {
  const textLen = xrGraphemes(pending.confirm ?? pending.label).length;
  const maxOffset = Math.max(0, Math.ceil(textLen / 36) - 6);
  return [
    {
      id: "confirm-action",
      label: m.s4_xr_panel_confirm_exec(),
      x: 100,
      y: 420,
      width: 380,
      height: 90,
      run: confirm,
    },
    {
      id: "cancel-action",
      label: m.s4_cancel(),
      x: 540,
      y: 420,
      width: 380,
      height: 90,
      run: cancel,
    },
    {
      id: "confirm-up",
      label: m.s4_xr_panel_confirm_up(),
      x: 100,
      y: 550,
      width: 380,
      height: 70,
      disabled: offset === 0,
      run: () => setOffset(Math.max(0, offset - 6)),
    },
    {
      id: "confirm-down",
      label: m.s4_xr_panel_confirm_down(),
      x: 540,
      y: 550,
      width: 380,
      height: 70,
      disabled: offset >= maxOffset,
      run: () => setOffset(Math.min(maxOffset, offset + 6)),
    },
  ];
}

export function drawUnavailableXrPanel(
  surface: XrSurface,
  layout: XrPanelLayout,
  close: XrRegion,
) {
  surface.regions = [
    {
      id: "move",
      label: m.s4_xr_panel_unavailable_title({
        session: layout.sessionId,
        view: layout.id.slice(-8),
      }),
      x: 8,
      y: 4,
      width: 800,
      height: 58,
      mode: "move",
    },
    close,
  ];
  surface.draw(() => {
    surface.text(m.s4_xr_panel_unavailable_line1(), 30, 140);
    surface.text(m.s4_xr_panel_unavailable_line2(), 30, 180);
  });
}
