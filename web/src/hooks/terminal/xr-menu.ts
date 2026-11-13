import type { XrPanelLayout } from "../../domains/terminal/xr-layout";
import type { XrWorkspaceOptions } from "../../domains/terminal/xr-workspace";
import { XrSurface, type XrRegion } from "./xr-surface";
import { xrGraphemes } from "../../domains/terminal/xr-text";

export function createXrMenu(
  options: XrWorkspaceOptions,
  layouts: () => readonly XrPanelLayout[],
  selected: () => string,
  select: (id: string) => void,
  toggle: (panel: XrPanelLayout) => void,
  reset: () => void,
  keyboard: () => void,
  notice: () => string,
  dismissNotice: () => void,
) {
  const surface = new XrSurface(800, 1450);
  surface.group.position.set(-1.35, 0, -1.75);
  surface.setSize(0.8, 1.45);
  let page = 0;
  let panelPage = 0;
  let signature = "";
  return {
    surface,
    update() {
      const sessions = options.available();
      const panels = layouts().filter(
        (panel) => panel.sessionId === selected(),
      );
      const maxPage = Math.max(1, Math.ceil(sessions.length / 6));
      const maxPanelPage = Math.max(1, Math.ceil(panels.length / 4));
      page = Math.min(page, maxPage - 1);
      panelPage = Math.min(panelPage, maxPanelPage - 1);
      const next = JSON.stringify([
        sessions,
        selected(),
        panels.map((panel) => [panel.id, panel.kind, panel.visible]),
        page,
        panelPage,
        notice(),
      ]);
      if (next === signature) return;
      signature = next;
      const regions: XrRegion[] = [];
      sessions.slice(page * 6, page * 6 + 6).forEach((session, index) =>
        regions.push({
          id: `session:${session.id}`,
          label: session.title,
          selected: selected() === session.id,
          x: 12,
          y: 62 + index * 64,
          width: 776,
          height: 60,
          run: () => {
            panelPage = 0;
            select(session.id);
            signature = "";
          },
        }),
      );
      regions.push(
        {
          id: "sessions-prev",
          label: "‹ 会话",
          x: 12,
          y: 452,
          width: 250,
          height: 60,
          disabled: page === 0,
          run: () => {
            page--;
            signature = "";
          },
        },
        {
          id: "sessions-next",
          label: "会话 ›",
          x: 538,
          y: 452,
          width: 250,
          height: 60,
          disabled: page === maxPage - 1,
          run: () => {
            page++;
            signature = "";
          },
        },
      );
      panels.slice(panelPage * 4, panelPage * 4 + 4).forEach((panel, index) =>
        regions.push({
          id: `panel:${panel.id}`,
          label: `${panel.visible ? "● 显示" : "○ 隐藏"} ${panel.kind} · ${panel.id.split(":").at(-1)}`,
          selected: panel.visible,
          x: 12,
          y: 572 + index * 64,
          width: 776,
          height: 60,
          run: () => {
            toggle(panel);
            signature = "";
          },
        }),
      );
      if (maxPanelPage > 1)
        regions.push({
          id: "panels-next",
          label: `面板 ${panelPage + 1}/${maxPanelPage} ›`,
          x: 12,
          y: 834,
          width: 776,
          height: 56,
          run: () => {
            panelPage = (panelPage + 1) % maxPanelPage;
            signature = "";
          },
        });
      regions.push(
        {
          id: "keyboard",
          label: "VR 键盘 / 收起键盘",
          x: 12,
          y: 904,
          width: 776,
          height: 70,
          run: keyboard,
        },
        {
          id: "reset",
          label: "恢复默认布局",
          x: 12,
          y: 984,
          width: 776,
          height: 70,
          run: reset,
        },
        {
          id: "exit",
          label: "退出 VR · 返回 2D",
          x: 12,
          y: 1064,
          width: 776,
          height: 80,
          run: options.exit,
        },
      );
      if (notice())
        regions.push({
          id: "dismiss-notice",
          label: "关闭提示",
          x: 12,
          y: 1368,
          width: 776,
          height: 60,
          run: dismissNotice,
        });
      surface.regions = regions;
      surface.draw(() => {
        surface.text("工作台 · 现有会话", 20, 16, "#edf4ef", "28px system-ui");
        surface.text(
          `${page + 1} / ${maxPage}`,
          330,
          470,
          "#a1ada5",
          "24px system-ui",
        );
        surface.text(
          "面板可见性 · 关闭不结束会话",
          20,
          534,
          "#edf4ef",
          "26px system-ui",
        );
        const message = xrGraphemes(notice());
        if (message.length) {
          for (let row = 0; row < 5; row++)
            surface.text(
              message.slice(row * 34, (row + 1) * 34).join(""),
              20,
              1160 + row * 36,
              "#f0a0a0",
              "22px system-ui",
            );
          if (message.length > 170)
            surface.text(
              "退出 VR 后可查看完整错误。",
              20,
              1340,
              "#f0a0a0",
              "20px system-ui",
            );
        } else
          surface.text(
            "选择会话后可通过面板聚焦实体键盘",
            20,
            1164,
            "#a1ada5",
            "22px system-ui",
          );
      });
    },
    dispose() {
      surface.dispose();
    },
  };
}
