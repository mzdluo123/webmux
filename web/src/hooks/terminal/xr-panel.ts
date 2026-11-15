import type { XrPanelLayout } from "../../domains/terminal/xr-layout";
import type {
  XrAction,
  XrSessionSource,
  XrTextPanel,
} from "../../domains/terminal/xr-workspace";
import { XrSurface, type XrRegion } from "./xr-surface";
import { XrTerminalMesh } from "./xr-terminal-mesh";
import { xrGraphemes } from "../../domains/terminal/xr-text";
import { xrGridControls } from "./xr-grid-controls";
import { drawXrPanelContent, drawUnavailableXrPanel } from "./xr-panel-content";

export class XrPanel {
  readonly surface = new XrSurface();
  private readonly terminalView = new XrTerminalMesh(this.surface.group);
  private text: XrTextPanel | undefined;
  private source: XrSessionSource | undefined;
  private offset = 0;
  private signature = "";
  private grid: [number, number] | undefined;
  private notice = "";
  private actionPage = 0;
  private pending: XrAction | undefined;
  private column = 0;
  private confirmationOffset = 0;
  private following = true;

  constructor(
    readonly layout: XrPanelLayout,
    private select: (id: string) => void,
    private changed: () => void,
    private exit: () => void,
    private error: (message: string) => void,
    private split?: (id: string) => void,
  ) {
    this.applyLayout();
  }

  applyLayout() {
    this.surface.group.position.fromArray(this.layout.position);
    this.surface.setSize(...this.layout.size);
    this.surface.group.visible = this.layout.visible;
    this.terminalView.fit(this.layout.size);
  }

  scroll(lines: number) {
    if (this.layout.kind === "terminal") this.source?.scroll(lines);
    else {
      this.offset = Math.max(
        0,
        Math.min(
          Math.max(0, (this.text?.lines.length ?? 0) - 18),
          this.offset + lines,
        ),
      );
      this.following = this.offset >= (this.text?.lines.length ?? 0) - 18;
    }
    this.signature = "";
  }

  tick() {
    if (this.layout.visible) this.terminalView.update(this.layout.size);
  }

  update(
    source: XrSessionSource,
    text: XrTextPanel | undefined,
    selected: boolean,
  ) {
    if (text?.kind === "chat" && this.following)
      this.offset = Math.max(0, text.lines.length - 18);
    this.source = source;
    this.text = text;
    if (this.layout.kind === "terminal")
      this.terminalView.sync(source.terminal, this.layout.size);
    const actions = this.bindingActions(text);
    const title = `${source.title}${text ? ` · ${text.title}` : ""} · 视图 ${this.layout.id.slice(-8)}`;
    if (this.pending)
      this.pending = actions.find(
        (action) => action.id === this.pending?.id && !action.disabled,
      );
    if (!this.layout.visible) return;
    this.terminalView.visible =
      this.layout.kind === "terminal" && !this.grid && !this.pending;
    if (text)
      this.offset = Math.min(this.offset, Math.max(0, text.lines.length - 18));
    const lines = text?.lines.slice(this.offset, this.offset + 18) ?? [];
    const signature = JSON.stringify([
      source.title,
      source.status,
      source.terminal?.cols,
      source.terminal?.rows,
      selected,
      source.graphicsUnsupported,
      source.canInput(this.layout.kind, this.layout.sourceId),
      text?.title,
      lines,
      actions.map((action) => [
        action.id,
        action.label,
        action.disabled,
        action.confirm,
      ]),
      this.grid,
      this.notice,
      this.actionPage,
      this.pending?.id,
      this.column,
      this.confirmationOffset,
    ]);
    if (signature === this.signature) return;
    this.signature = signature;
    const regions: XrRegion[] = [
      {
        id: "body",
        x: 20,
        y: 110,
        width: 960,
        height: 530,
        mode: "scroll",
        run: () => this.activate(),
      },
      {
        id: "move",
        label: `${selected ? "● " : ""}${title} · ${source.status} · 拖动`,
        x: 8,
        y: 4,
        width: 800,
        height: 58,
        mode: "move",
        selected,
        run: () => this.activate(),
      },
      this.closeRegion(),
      {
        id: "up",
        label: "↑ 历史",
        x: 8,
        y: 64,
        width: 155,
        height: 46,
        run: () => this.scroll(-8),
      },
      {
        id: "down",
        label: "↓",
        x: 165,
        y: 64,
        width: 80,
        height: 46,
        run: () => this.scroll(8),
      },
      {
        id: "latest",
        label: "最新",
        x: 247,
        y: 64,
        width: 130,
        height: 46,
        run: () => {
          if (this.layout.kind === "terminal") this.source?.latest();
          this.offset = Math.max(0, (this.text?.lines.length ?? 0) - 18);
          this.following = true;
          this.signature = "";
        },
      },
      {
        id: "focus",
        label: "实体键盘",
        x: 379,
        y: 64,
        width: 190,
        height: 46,
        run: () => this.activate(),
        selected,
      },
      {
        id: "resize",
        label: "↘ 缩放",
        x: 882,
        y: 712,
        width: 134,
        height: 48,
        mode: "resize",
      },
    ];
    if (this.layout.kind === "terminal") {
      regions.push({
        id: "grid",
        label: this.grid
          ? `网格 ${this.grid[0]}×${this.grid[1]}`
          : `网格 ${source.terminal?.cols ?? "—"}×${source.terminal?.rows ?? "—"}`,
        x: 572,
        y: 64,
        width: 310,
        height: 46,
        run: () => {
          this.grid = [
            source.terminal?.cols ?? 80,
            source.terminal?.rows ?? 24,
          ];
          this.signature = "";
        },
      });
      if (this.grid) this.gridControls(regions);
    }
    if (text) {
      const maxColumn = text.lines.reduce(
        (longest, line) =>
          Math.max(
            longest,
            xrGraphemes(line.text.replaceAll("\t", "    ")).length - 40,
          ),
        0,
      );
      this.column = Math.min(this.column, maxColumn);
      regions.push(
        {
          id: "text-left",
          label: "← 内容",
          x: 572,
          y: 64,
          width: 150,
          height: 46,
          disabled: this.column === 0,
          run: () => {
            this.column = Math.max(0, this.column - 32);
            this.signature = "";
          },
        },
        {
          id: "text-right",
          label: "内容 →",
          x: 724,
          y: 64,
          width: 158,
          height: 46,
          disabled: this.column >= maxColumn,
          run: () => {
            this.column = Math.min(maxColumn, this.column + 32);
            this.signature = "";
          },
        },
      );
    }
    const pageCount = Math.max(1, Math.ceil(actions.length / 3));
    this.actionPage = Math.min(this.actionPage, pageCount - 1);
    actions
      .slice(this.actionPage * 3, this.actionPage * 3 + 3)
      .forEach((action, index) =>
        regions.push({
          id: `action:${action.id}`,
          label: action.label,
          disabled: action.disabled,
          x: 8 + index * 270,
          y: 650,
          width: 266,
          height: 56,
          run: () => {
            const current = this.currentAction(action.id);
            if (!current || current.disabled) return;
            this.activate();
            if (current.confirm) {
              this.pending = current;
              this.confirmationOffset = 0;
              this.signature = "";
            } else this.run(current.id);
          },
        }),
      );
    if (pageCount > 1)
      regions.push({
        id: "actions-next",
        label: `${this.actionPage + 1}/${pageCount} ›`,
        x: 820,
        y: 650,
        width: 196,
        height: 56,
        run: () => {
          this.actionPage = (this.actionPage + 1) % pageCount;
          this.signature = "";
        },
      });
    if (source.graphicsUnsupported)
      regions.push({
        id: "graphics",
        label: "图形内容不支持 VR · 返回 2D 查看",
        x: 8,
        y: 712,
        width: 870,
        height: 48,
        run: () => {
          this.activate();
          this.exit();
        },
      });
    if (this.pending) {
      const pending = this.pending;
      const maxOffset = Math.max(
        0,
        Math.ceil(xrGraphemes(pending.confirm ?? pending.label).length / 36) -
          6,
      );
      regions.length = 0;
      regions.push(
        {
          id: "confirm-action",
          label: "确认执行",
          x: 100,
          y: 420,
          width: 380,
          height: 90,
          run: () => {
            const current = this.currentAction(pending.id);
            this.pending = undefined;
            this.signature = "";
            if (current && !current.disabled) this.run(current.id);
          },
        },
        {
          id: "cancel-action",
          label: "取消",
          x: 540,
          y: 420,
          width: 380,
          height: 90,
          run: () => {
            this.pending = undefined;
            this.signature = "";
          },
        },
        {
          id: "confirm-up",
          label: "↑ 说明",
          x: 100,
          y: 550,
          width: 380,
          height: 70,
          disabled: this.confirmationOffset === 0,
          run: () => {
            this.confirmationOffset = Math.max(0, this.confirmationOffset - 6);
            this.signature = "";
          },
        },
        {
          id: "confirm-down",
          label: "↓ 说明",
          x: 540,
          y: 550,
          width: 380,
          height: 70,
          disabled: this.confirmationOffset >= maxOffset,
          run: () => {
            this.confirmationOffset = Math.min(
              maxOffset,
              this.confirmationOffset + 6,
            );
            this.signature = "";
          },
        },
      );
    }
    if (
      !this.pending &&
      this.layout.kind === "chat" &&
      this.layout.sourceId === "chat" &&
      this.split
    )
      regions.push({
        id: "split",
        label: "拆分视图",
        x: 572,
        y: 712,
        width: 306,
        height: 48,
        run: () => this.split?.(this.layout.id),
      });
    this.surface.regions = regions;
    this.surface.draw(() =>
      drawXrPanelContent(this.surface, source, this.layout, {
        pending: this.pending,
        confirmationOffset: this.confirmationOffset,
        lines,
        column: this.column,
        notice: this.notice,
      }),
    );
  }

  private activate() {
    this.select(this.layout.id);
  }

  private bindingActions(text: XrTextPanel | undefined) {
    const readonlyChat =
      this.layout.kind === "chat" &&
      this.layout.sourceId === "chat" &&
      !this.source?.canInput("chat", "chat");
    return [
      ...(text?.actions ?? []),
      ...(readonlyChat ? [] : (this.source?.actions() ?? [])),
    ];
  }

  private currentAction(id: string) {
    if (!this.layout.visible || !this.source) return undefined;
    const text = this.source
      ?.panels()
      .find(
        (panel) =>
          panel.kind === this.layout.kind && panel.id === this.layout.sourceId,
      );
    return this.bindingActions(text).find((action) => action.id === id);
  }

  private run(id: string) {
    this.activate();
    void Promise.resolve()
      .then(() => {
        const current = this.currentAction(id);
        if (current && !current.disabled) return current.run();
      })
      .catch((cause: unknown) => this.error(String(cause)));
  }

  private gridControls(regions: XrRegion[]) {
    if (!this.grid) return;
    regions.push(
      ...xrGridControls(
        this.grid,
        (grid) => {
          if (grid)
            this.notice = this.source?.resize(...grid)
              ? "网格已提交"
              : "观察端无法修改网格；请先取得控制权";
          this.grid = undefined;
          this.signature = "";
        },
        () => {
          this.signature = "";
        },
      ),
    );
    // Grid controls are a foreground overlay, not part of the terminal texture.
    this.terminalView.visible = false;
  }

  private closeRegion(): XrRegion {
    return {
      id: "close",
      label: "关闭",
      x: 812,
      y: 4,
      width: 204,
      height: 58,
      run: () => {
        this.layout.visible = false;
        this.pending = undefined;
        this.applyLayout();
        this.changed();
      },
    };
  }

  detach() {
    if (this.signature === "unavailable") {
      this.applyLayout();
      return;
    }
    this.terminalView.dispose();
    this.source = undefined;
    this.text = undefined;
    this.pending = undefined;
    this.grid = undefined;
    this.signature = "unavailable";
    this.applyLayout();
    drawUnavailableXrPanel(this.surface, this.layout, this.closeRegion());
  }

  dispose() {
    this.source = undefined;
    this.text = undefined;
    this.pending = undefined;
    this.terminalView.dispose();
    this.surface.dispose();
  }
}
