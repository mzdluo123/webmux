import { locale } from "$lib/locale";
import { m } from "$lib/paraglide/messages.js";
import type {
  XrTextPanel,
  XrWorkspaceOptions,
} from "../../domains/terminal/xr-workspace";
import type { XrViewState } from "./xr-view-state";
import { XrSurface, type XrRegion } from "./xr-surface";
import { xrGraphemes } from "../../domains/terminal/xr-text";

export function createXrMenu(
  options: XrWorkspaceOptions,
  views: XrViewState,
  keyboard: () => void,
  notice: () => string,
  dismissNotice: () => void,
) {
  const surface = new XrSurface(800, 1450);
  surface.group.position.set(-1.35, 0, -1.75);
  surface.setSize(0.8, 1.45);
  let sessionId = options.initialId;
  let page = 0;
  let panelPage = 0;
  let signature = "";
  return {
    surface,
    update(texts: ReadonlyMap<string, readonly XrTextPanel[]>) {
      const sessions = options.available();
      const contentSessionId = sessionId;
      const bindings = views.layouts.filter(
        (panel) => panel.sessionId === sessionId,
      );
      const contents = [
        {
          kind: "terminal" as const,
          id: "terminal",
          title: m.s4_tab_terminal(),
        },
        ...(texts.get(sessionId) ?? []),
      ].filter(
        (text) =>
          !bindings.some(
            (panel) => panel.kind === text.kind && panel.sourceId === text.id,
          ),
      );
      const entries = [
        ...bindings.map((panel) => ({
          id: `panel:${panel.id}`,
          label: panel.visible
            ? m.s4_xr_menu_panel_visible({
                kind: panel.kind,
                source: panel.sourceId,
                view: panel.id.slice(-8),
              })
            : m.s4_xr_menu_panel_hidden({
                kind: panel.kind,
                source: panel.sourceId,
                view: panel.id.slice(-8),
              }),
          selected: panel.id === views.selectedViewId,
          run: () => views.toggle(panel),
        })),
        ...contents.map((text) => ({
          id: `content:${text.kind}:${text.id}`,
          label: m.s4_xr_menu_open_content({
            title: text.title,
            kind: text.id === "notice" ? m.s4_xr_menu_notice_kind() : text.kind,
          }),
          selected: false,
          run: () => views.openContent(contentSessionId, text.kind, text.id),
        })),
      ];
      const maxPage = Math.max(1, Math.ceil(sessions.length / 6));
      const maxPanelPage = Math.max(1, Math.ceil(entries.length / 4));
      page = Math.min(page, maxPage - 1);
      panelPage = Math.min(panelPage, maxPanelPage - 1);
      const next = JSON.stringify([
        locale.current,
        sessions.map((item) => [
          item.id,
          item.title,
          views.observed.has(item.id),
          views.pending.has(item.id),
          options.sources().find((source) => source.id === item.id)?.status,
        ]),
        sessionId,
        entries.map((entry) => [entry.id, entry.label, entry.selected]),
        views.selectedViewId,
        page,
        panelPage,
        notice(),
      ]);
      if (next === signature) return;
      signature = next;
      const regions: XrRegion[] = [];
      sessions.slice(page * 6, page * 6 + 6).forEach((item, index) => {
        const observed = views.observed.has(item.id);
        const pending = views.pending.has(item.id);
        const status = options
          .sources()
          .find((source) => source.id === item.id)?.status;
        regions.push(
          {
            id: `session:${item.id}`,
            label: `${pending ? m.s4_xr_menu_joining() : observed ? m.s4_xr_menu_observing() : m.s4_xr_menu_join()} ${item.title}${status ? ` · ${status}` : ""}`,
            selected: sessionId === item.id,
            disabled: pending,
            x: 12,
            y: 62 + index * 64,
            width: 578,
            height: 60,
            run: () => {
              sessionId = item.id;
              panelPage = 0;
              if (!observed) void views.observe(item.id);
              signature = "";
            },
          },
          {
            id: `unobserve:${item.id}`,
            label: m.s4_xr_menu_unobserve(),
            disabled: !observed && !pending,
            x: 594,
            y: 62 + index * 64,
            width: 194,
            height: 60,
            run: () => views.remove(item.id),
          },
        );
      });
      regions.push(
        {
          id: "sessions-prev",
          label: m.s4_xr_menu_sessions_prev(),
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
          label: m.s4_xr_menu_sessions_next(),
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
      entries.slice(panelPage * 4, panelPage * 4 + 4).forEach((entry, index) =>
        regions.push({
          ...entry,
          x: 12,
          y: 572 + index * 64,
          width: 776,
          height: 60,
        }),
      );
      if (maxPanelPage > 1)
        regions.push({
          id: "panels-next",
          label: m.s4_xr_menu_panels_next({
            page: String(panelPage + 1),
            total: String(maxPanelPage),
          }),
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
          label: m.s4_xr_menu_toggle_keyboard(),
          x: 12,
          y: 904,
          width: 776,
          height: 70,
          run: keyboard,
        },
        {
          id: "reset",
          label: m.s4_xr_menu_reset_layout(),
          x: 12,
          y: 984,
          width: 776,
          height: 70,
          run: views.reset,
        },
        {
          id: "exit",
          label: m.s4_xr_menu_exit_vr(),
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
          label: m.s4_dismiss_notice(),
          x: 12,
          y: 1368,
          width: 776,
          height: 60,
          run: dismissNotice,
        });
      surface.regions = regions;
      surface.draw(() => {
        surface.text(
          m.s4_xr_menu_header(),
          20,
          16,
          "#edf4ef",
          "28px system-ui",
        );
        surface.text(
          `${page + 1} / ${maxPage}`,
          330,
          470,
          "#a1ada5",
          "24px system-ui",
        );
        surface.text(
          m.s4_xr_menu_panels_header(),
          20,
          534,
          "#edf4ef",
          "26px system-ui",
        );
        const target = views.selection();
        const message = xrGraphemes(
          notice() ||
            (target
              ? m.s4_xr_menu_input_target({
                  title: target.source.title,
                  view: target.layout.id.slice(-8),
                })
              : m.s4_xr_menu_input_none()),
        );
        for (let row = 0; row < 5; row++)
          surface.text(
            message.slice(row * 34, (row + 1) * 34).join(""),
            20,
            1160 + row * 36,
            notice() ? "#f0a0a0" : "#a1ada5",
            "22px system-ui",
          );
        if (message.length > 170)
          surface.text(
            m.s4_xr_menu_full_error_in_2d(),
            20,
            1340,
            "#f0a0a0",
            "20px system-ui",
          );
      });
    },
    dispose() {
      surface.dispose();
    },
  };
}
