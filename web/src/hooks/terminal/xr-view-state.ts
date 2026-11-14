import {
  boundXrPanel,
  defaultXrPanel,
  XR_PANEL_LIMIT,
  type XrPanelLayout,
} from "../../domains/terminal/xr-layout";
import type {
  XrSessionSource,
  XrTextPanel,
  XrWorkspaceOptions,
} from "../../domains/terminal/xr-workspace";

export interface XrViewSelection {
  source: XrSessionSource;
  layout: XrPanelLayout;
}

export interface XrViewState {
  readonly layouts: readonly XrPanelLayout[];
  readonly selectedViewId: string | undefined;
  readonly observed: ReadonlySet<string>;
  readonly pending: ReadonlyMap<string, Promise<void>>;
  selection(this: void): XrViewSelection | undefined;
  select(this: void, id: string): void;
  observe(id: string, createDefault?: boolean): Promise<void>;
  remove(id: string): void;
  toggle(layout: XrPanelLayout): void;
  openContent(id: string, kind: XrPanelLayout["kind"], sourceId: string): void;
  split(this: void, id: string): void;
  reconcile(): void;
  refresh(): {
    sources: readonly XrSessionSource[];
    texts: ReadonlyMap<string, readonly XrTextPanel[]>;
  };
  reset(this: void): void;
  initialize(): Promise<void>;
  dispose(): void;
}

/** Binding lifecycle and capacity transactions, independent of the scene renderer. */
export function createXrViewState(
  options: XrWorkspaceOptions,
  initial: XrPanelLayout[] | undefined,
  changed: () => void,
  report: (message: string) => void,
): XrViewState {
  let layouts = initial ?? [];
  let selectedViewId: string | undefined;
  let disposed = false;
  const observed = new Set<string>();
  const pending = new Map<string, Promise<void>>();
  const reservations = new Set<string | object>();
  const cancelled = new Set<string>();
  const primaryWaiters = new Map<string, () => void>();
  let focused: XrSessionSource | undefined;
  let focusedView: string | undefined;
  const texts = new Map<string, readonly XrTextPanel[]>();

  function source(id: string) {
    return options.sources().find((item) => item.id === id);
  }
  function selection() {
    const layout = layouts.find(
      (item) => item.id === selectedViewId && item.visible,
    );
    const current = layout && source(layout.sessionId);
    if (
      !layout ||
      !current ||
      (layout.kind !== "terminal" &&
        !texts
          .get(current.id)
          ?.some(
            (text) => text.kind === layout.kind && text.id === layout.sourceId,
          ))
    )
      return undefined;
    return { layout, source: current };
  }
  function clearFocus() {
    focused?.blur();
    focused = undefined;
    focusedView = undefined;
  }
  function syncFocus() {
    const target = selection();
    if (
      !target ||
      (focused && focused !== target.source) ||
      !target.source.canInput(target.layout.kind, target.layout.sourceId)
    ) {
      selectedViewId = undefined;
      clearFocus();
      return;
    }
    if (focused === target.source && focusedView === target.layout.id) return;
    clearFocus();
    const owner = target.source;
    const id = target.layout.id;
    focused = owner;
    focusedView = id;
    const isCurrent = () =>
      !disposed &&
      selectedViewId === id &&
      selection()?.source === owner &&
      owner.canInput(target.layout.kind, target.layout.sourceId);
    void Promise.resolve(
      owner.focus(target.layout.kind, target.layout.sourceId, isCurrent),
    ).catch((cause: unknown) => report(String(cause)));
  }
  function select(id: string) {
    const layout = layouts.find((item) => item.id === id && item.visible);
    if (!layout) return;
    if (selectedViewId !== id || focused !== source(layout.sessionId))
      clearFocus();
    selectedViewId = id;
    syncFocus();
    changed();
  }
  function releaseUnused() {
    for (const id of observed) {
      if (layouts.some((layout) => layout.sessionId === id) || pending.has(id))
        continue;
      observed.delete(id);
      options.unobserve(id);
    }
  }
  function reserve() {
    if (
      layouts.length +
        [...reservations].filter(
          (slot) =>
            typeof slot !== "string" ||
            !layouts.some((layout) => layout.id === slot),
        ).length <
      XR_PANEL_LIMIT
    ) {
      const slot = {};
      reservations.add(slot);
      return slot;
    }
    const hidden = layouts.find(
      (layout) => !layout.visible && !reservations.has(layout.id),
    );
    if (hidden) {
      reservations.add(hidden.id);
      return hidden.id;
    }
    report("VR 最多显示 16 个面板；请关闭一个面板后再打开内容。");
    return undefined;
  }
  function commit(layout: XrPanelLayout, slot: string | object) {
    if (typeof slot === "string")
      layouts = layouts.filter((item) => item.id !== slot);
    reservations.delete(slot);
    layouts.push(layout);
    releaseUnused();
    changed();
  }
  function make(id: string, kind: XrPanelLayout["kind"], sourceId: string) {
    const viewId = Array.from(
      crypto.getRandomValues(new Uint32Array(4)),
      (word) => word.toString(16),
    ).join("-");
    return defaultXrPanel(viewId, id, kind, layouts.length + 1, sourceId);
  }
  function main(id: string) {
    const kind = source(id)?.primaryKind ?? "terminal";
    return make(id, kind, kind);
  }
  function observe(id: string, createDefault = true): Promise<void> {
    const inflight = pending.get(id);
    if (inflight) return inflight;
    if (observed.has(id)) {
      if (createDefault) {
        const existing = layouts.find((layout) => layout.sessionId === id);
        if (existing) {
          if (!existing.visible) toggle(existing);
          else select(existing.id);
        }
      }
      return Promise.resolve();
    }
    const existing = layouts.find((layout) => layout.sessionId === id);
    const slot = createDefault && !existing ? reserve() : undefined;
    if (createDefault && !existing && !slot) return Promise.resolve();
    cancelled.delete(id);
    const operation = Promise.resolve()
      .then(async () => {
        await options.observe(id);
        while (
          slot &&
          !source(id)?.primaryKind &&
          !disposed &&
          !cancelled.has(id) &&
          !(
            options.ready() &&
            !options.available().some((item) => item.id === id)
          )
        ) {
          const { promise, resolve } = Promise.withResolvers<void>();
          primaryWaiters.set(id, resolve);
          await promise;
          primaryWaiters.delete(id);
        }
        if (
          disposed ||
          cancelled.has(id) ||
          (options.ready() &&
            !options.available().some((item) => item.id === id))
        ) {
          options.unobserve(id);
          return;
        }
        observed.add(id);
        const current = source(id);
        if (current) texts.set(id, current.panels());
        if (slot) {
          const layout = main(id);
          commit(layout, slot);
          select(layout.id);
        } else if (existing && createDefault && layouts.includes(existing)) {
          if (!existing.visible) toggle(existing);
          else select(existing.id);
        }
      })
      .catch((cause: unknown) => {
        observed.delete(id);
        options.unobserve(id);
        report(`无法观察会话：${String(cause)}`);
      })
      .finally(() => {
        if (slot) reservations.delete(slot);
        pending.delete(id);
        releaseUnused();
      });
    pending.set(id, operation);
    return operation;
  }
  function remove(id: string) {
    if (pending.has(id)) cancelled.add(id);
    primaryWaiters.get(id)?.();
    layouts = layouts.filter((layout) => layout.sessionId !== id);
    if (!layouts.some((layout) => layout.id === selectedViewId)) {
      selectedViewId = undefined;
      clearFocus();
    }
    releaseUnused();
    changed();
  }
  function toggle(layout: XrPanelLayout) {
    if (!layouts.includes(layout)) return;
    if (!layout.visible && reservations.has(layout.id)) {
      report("该隐藏槽位正在用于加入观察；请等待完成。");
      return;
    }
    layout.visible = !layout.visible;
    if (!layout.visible && selectedViewId === layout.id) {
      selectedViewId = undefined;
      clearFocus();
    }
    if (layout.visible) select(layout.id);
    changed();
  }
  function openContent(
    id: string,
    kind: XrPanelLayout["kind"],
    sourceId: string,
  ) {
    const existing = layouts.find(
      (layout) =>
        layout.sessionId === id &&
        layout.kind === kind &&
        layout.sourceId === sourceId,
    );
    if (existing) {
      if (!existing.visible) toggle(existing);
      else select(existing.id);
      return;
    }
    if (!observed.has(id)) return;
    if (
      kind !== "terminal" &&
      !texts.get(id)?.some((text) => text.kind === kind && text.id === sourceId)
    )
      return;
    const slot = reserve();
    if (!slot) return;
    const layout = make(id, kind, sourceId);
    commit(layout, slot);
    select(layout.id);
  }
  function split(id: string) {
    const original = layouts.find(
      (layout) =>
        layout.id === id &&
        layout.visible &&
        layout.kind === "chat" &&
        layout.sourceId === "chat",
    );
    if (
      !original ||
      !texts
        .get(original.sessionId)
        ?.some((text) => text.kind === "chat" && text.id === "chat")
    )
      return;
    const slot = reserve();
    if (!slot) return;
    const clone = make(original.sessionId, "chat", "chat");
    clone.position = [
      original.position[0] + 0.35,
      original.position[1] - 0.2,
      original.position[2] + 0.08,
    ];
    clone.size = [...original.size];
    boundXrPanel(clone);
    commit(clone, slot);
    select(clone.id);
  }
  function reconcile() {
    if (selectedViewId && !selection()) {
      selectedViewId = undefined;
      clearFocus();
    }
    syncFocus();
  }
  function refresh() {
    if (options.ready()) {
      const available = new Set(options.available().map((item) => item.id));
      for (const id of new Set(layouts.map((layout) => layout.sessionId)))
        if (!available.has(id)) remove(id);
    }
    const available = new Set(options.available().map((item) => item.id));
    for (const id of new Set(layouts.map((layout) => layout.sessionId)))
      if (available.has(id) && !observed.has(id) && !pending.has(id))
        void observe(id, false);
    const sources = options.sources();
    texts.clear();
    for (const current of sources) texts.set(current.id, current.panels());
    for (const [id, resolve] of primaryWaiters)
      if (
        source(id)?.primaryKind ||
        cancelled.has(id) ||
        (options.ready() && !available.has(id))
      )
        resolve();
    reconcile();
    return { sources, texts };
  }
  function reset() {
    if (pending.size) {
      report("正在加入观察会话；完成后可恢复默认布局。");
      return;
    }
    if ([...observed].some((id) => source(id)?.primaryKind === undefined)) {
      report("观察会话内容类型尚未就绪；请等待后再恢复默认布局。");
      return;
    }
    layouts = [...observed].map((id, index) => {
      const layout = main(id);
      layout.position = defaultXrPanel(
        layout.id,
        id,
        layout.kind,
        index + 1,
        layout.sourceId,
      ).position;
      return layout;
    });
    selectedViewId = undefined;
    clearFocus();
    changed();
  }
  async function initialize() {
    if (initial === undefined) await observe(options.initialId);
    else if (options.ready())
      await Promise.all(
        [...new Set(layouts.map((layout) => layout.sessionId))]
          .filter((id) => options.available().some((item) => item.id === id))
          .map((id) => observe(id, false)),
      );
  }
  return {
    get layouts() {
      return layouts;
    },
    get selectedViewId() {
      return selectedViewId;
    },
    observed,
    pending,
    selection,
    select,
    observe,
    remove,
    toggle,
    openContent,
    split,
    reconcile,
    refresh,
    reset,
    initialize,
    dispose() {
      disposed = true;
      selectedViewId = undefined;
      clearFocus();
      for (const resolve of primaryWaiters.values()) resolve();
      primaryWaiters.clear();
      for (const id of observed) options.unobserve(id);
      observed.clear();
    },
  };
}
