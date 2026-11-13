import { Color, PerspectiveCamera, Scene, WebGLRenderer } from "three";
import { z } from "zod";
import {
  decodeXrLayout,
  defaultXrPanel,
  encodeXrLayout,
  XR_PANEL_LIMIT,
  type XrPanelLayout,
} from "../../domains/terminal/xr-layout";
import type {
  XrWorkspaceOptions,
  XrWorkspaceRuntime,
} from "../../domains/terminal/xr-workspace";
import { deviceObjectPreference } from "../shared/device-preferences.svelte";
import { createXrKeyboard } from "./xr-keyboard";
import { createXrMenu } from "./xr-menu";
import { XrPanel } from "./xr-panel";
import { createXrPointers } from "./xr-pointer";
import { orderXrSurfaces } from "./xr-surface";

/** Invoked only after the owning coordinator obtains a user-initiated XRSession. */
export async function startXrWorkspace(
  session: XRSession,
  options: XrWorkspaceOptions,
): Promise<XrWorkspaceRuntime> {
  const resources: (() => void)[] = [];
  try {
    return await createWorkspace(session, options, resources);
  } catch (cause) {
    releaseResources(resources, options);
    throw cause;
  }
}

function releaseResources(
  resources: (() => void)[],
  options: XrWorkspaceOptions,
) {
  for (const release of resources.splice(0).reverse()) {
    try {
      release();
    } catch (cause) {
      options.error(`VR 资源清理失败：${String(cause)}`);
    }
  }
}

async function createWorkspace(
  session: XRSession,
  options: XrWorkspaceOptions,
  resources: (() => void)[],
): Promise<XrWorkspaceRuntime> {
  let notice = "";
  function report(message: string) {
    notice = message;
    options.error(message);
  }
  const canvas = document.createElement("canvas");
  canvas.hidden = true;
  canvas.setAttribute("aria-hidden", "true");
  const renderer = new WebGLRenderer({ canvas, antialias: true, alpha: false });
  function releaseRenderer() {
    renderer.xr.removeEventListener("sessionend", releaseRenderer);
    renderer.renderLists.dispose();
    renderer.dispose();
    renderer.forceContextLoss();
  }
  resources.push(() => {
    renderer.setAnimationLoop(null);
    canvas.remove();
    // Three restores its render target and detaches session listeners on end,
    // before announcing sessionend. Never dispose its context before that.
    if (renderer.xr.getSession())
      renderer.xr.addEventListener("sessionend", releaseRenderer);
    else releaseRenderer();
  });
  renderer.xr.enabled = true;
  renderer.xr.setReferenceSpaceType("local");
  renderer.setPixelRatio(1);
  renderer.setSize(1024, 768, false);
  document.body.append(canvas);
  const scene = new Scene();
  scene.background = new Color("#080c0a");
  const camera = new PerspectiveCamera(60, 1, 0.05, 30);
  const knownIds = new Set(options.available().map((source) => source.id));
  for (const source of options.sources()) knownIds.add(source.id);
  const preference = deviceObjectPreference(
    "xr-workspace-layout",
    z.unknown().transform((value) => ({
      version: 1 as const,
      panels: decodeXrLayout(value, knownIds),
    })),
    () => ({ version: 1 as const, panels: [] }),
  );
  let layouts: XrPanelLayout[] = preference.value.panels;
  let selectedId = options.initialId;
  let focusRequested = true;
  let disposed = false;
  let nextRefresh = 0;
  let layoutChanged = false;
  const panels = new Map<string, XrPanel>();
  resources.push(() => {
    for (const panel of panels.values()) panel.dispose();
    panels.clear();
  });
  const opened = new Set(options.sources().map((source) => source.id));
  function save() {
    preference.set(encodeXrLayout(layouts));
    layoutChanged = false;
  }
  function requestSource(id: string) {
    if (opened.has(id) || options.sources().some((source) => source.id === id))
      return;
    opened.add(id);
    void Promise.resolve()
      .then(() => options.open(id))
      .catch((cause: unknown) => {
        report(`无法打开会话：${String(cause)}`);
      });
  }
  function activateSource(id: string) {
    selectedId = id;
    focusRequested = true;
    if (options.sources().some((source) => source.id === id))
      void Promise.resolve()
        .then(() => options.open(id))
        .catch((cause: unknown) => report(String(cause)));
    else requestSource(id);
    nextRefresh = 0;
  }
  function select(id: string) {
    let terminal = layouts.find(
      (panel) => panel.sessionId === id && panel.kind === "terminal",
    );
    if (!terminal) {
      terminal = defaultXrPanel(
        `terminal:${id}`,
        id,
        "terminal",
        layouts.length + 1,
      );
      if (!add(terminal)) return;
    }
    if (!options.sources().some((source) => source.id === id))
      opened.delete(id);
    activateSource(id);
    terminal.visible = true;
    panels.get(terminal.id)?.applyLayout();
    options
      .sources()
      .find((source) => source.id === id)
      ?.focus();
    save();
    nextRefresh = 0;
  }
  function add(layout: XrPanelLayout, initial = false): boolean {
    if (layouts.length >= XR_PANEL_LIMIT) {
      let slot = layouts.findIndex(
        (saved) =>
          !saved.visible ||
          panels.get(saved.id)?.surface.group.visible === false,
      );
      if (slot < 0 && initial) slot = layouts.length - 1;
      if (slot < 0) {
        report("VR 最多显示 16 个面板；请关闭一个面板后再打开内容。");
        return false;
      }
      const removed = layouts.splice(slot, 1)[0]!;
      panels.get(removed.id)?.dispose();
      panels.delete(removed.id);
      layout.position = [...removed.position];
      layout.size = [...removed.size];
    }
    layouts.push(layout);
    layoutChanged = true;
    return true;
  }
  if (
    !layouts.some(
      (panel) => panel.sessionId === selectedId && panel.kind === "terminal",
    )
  ) {
    add(
      defaultXrPanel(
        `terminal:${selectedId}`,
        selectedId,
        "terminal",
        layouts.length + 1,
      ),
      true,
    );
  }
  const keyboard = createXrKeyboard(() =>
    options.sources().find((source) => source.id === selectedId),
  );
  resources.push(() => keyboard.dispose());
  const menu = createXrMenu(
    options,
    () => layouts,
    () => selectedId,
    select,
    (layout) => {
      layout.visible = !layout.visible;
      if (layout.visible) requestSource(layout.sessionId);
      panels.get(layout.id)?.applyLayout();
      save();
    },
    () => {
      for (const panel of panels.values()) panel.dispose();
      panels.clear();
      layouts = [];
      add(defaultXrPanel(`terminal:${selectedId}`, selectedId, "terminal", 1));
      save();
      nextRefresh = 0;
    },
    () => {
      keyboard.surface.group.visible = !keyboard.surface.group.visible;
    },
    () => notice,
    () => {
      notice = "";
    },
  );
  resources.push(() => menu.dispose());
  scene.add(menu.surface.group, keyboard.surface.group);
  let pointers:
    | { dispose(): void; update(frame: XRFrame, now: number): void }
    | undefined;
  resources.push(() => pointers?.dispose());
  const surfaces = [menu.surface, keyboard.surface];

  function dispose() {
    if (disposed) return;
    disposed = true;
    save();
    renderer.setAnimationLoop(null);
    releaseResources(resources, options);
    scene.clear();
  }
  function contextLost(event: Event) {
    event.preventDefault();
    report("VR 图形上下文已丢失；已返回 2D，会话仍在运行。请重新进入 VR。");
    options.exit();
    dispose();
  }
  session.addEventListener("end", dispose);
  canvas.addEventListener("webglcontextlost", contextLost);
  resources.push(() => {
    session.removeEventListener("end", dispose);
    canvas.removeEventListener("webglcontextlost", contextLost);
  });

  function refresh() {
    const sources = options.sources();
    if (focusRequested) {
      const selected = sources.find((source) => source.id === selectedId);
      if (selected) {
        selected.focus();
        focusRequested = false;
      }
    }
    const available = new Set(options.available().map((source) => source.id));
    for (const source of sources) available.add(source.id);
    knownIds.clear();
    for (const id of available) knownIds.add(id);
    for (const layout of layouts)
      if (layout.visible && available.has(layout.sessionId))
        requestSource(layout.sessionId);
    for (const source of sources) {
      if (
        !layouts.some(
          (layout) =>
            layout.sessionId === source.id && layout.kind === "terminal",
        )
      )
        add(
          defaultXrPanel(
            `terminal:${source.id}`,
            source.id,
            "terminal",
            layouts.length + 1,
          ),
        );
      const textPanels = source.panels();
      for (const text of textPanels) {
        const id = `${text.kind}:${source.id}:${text.id}`;
        if (!layouts.some((layout) => layout.id === id))
          add(defaultXrPanel(id, source.id, text.kind, layouts.length + 1));
      }
      for (const layout of layouts.filter(
        (layout) => layout.sessionId === source.id,
      )) {
        const text = textPanels.find(
          (item) => `${item.kind}:${source.id}:${item.id}` === layout.id,
        );
        if (layout.kind !== "terminal" && !text) {
          const stale = panels.get(layout.id);
          if (stale) stale.surface.group.visible = false;
          continue;
        }
        let panel = panels.get(layout.id);
        if (!panel) {
          panel = new XrPanel(
            layout,
            activateSource,
            save,
            options.exit,
            report,
          );
          panels.set(layout.id, panel);
          scene.add(panel.surface.group);
        }
        panel.applyLayout();
        panel.update(source, text, selectedId === source.id);
      }
    }
    for (const [id, panel] of panels) {
      if (!available.has(panel.layout.sessionId)) {
        panel.dispose();
        panels.delete(id);
      } else if (
        !sources.some((source) => source.id === panel.layout.sessionId)
      )
        panel.detach();
    }
    const remaining = layouts.filter((layout) =>
      available.has(layout.sessionId),
    );
    if (remaining.length !== layouts.length) layoutChanged = true;
    layouts = remaining;
    if (layoutChanged) save();
    surfaces.length = 2;
    for (const panel of panels.values()) surfaces.push(panel.surface);
    orderXrSurfaces(surfaces);
    menu.update();
    keyboard.update();
  }

  try {
    await renderer.xr.setSession(session);
    if (disposed) throw new Error("VR 会话已结束");
    const reference = renderer.xr.getReferenceSpace();
    if (!reference) throw new Error("VR local 参考空间不可用");
    pointers = createXrPointers(
      session,
      scene,
      reference,
      () => surfaces,
      (surface) => {
        for (const panel of panels.values())
          if (panel.surface === surface) return panel;
        return undefined;
      },
      save,
    );
    requestSource(selectedId);
    refresh();
    renderer.setAnimationLoop((now, frame) => {
      if (disposed || !frame || session.visibilityState !== "visible") return;
      try {
        if (now >= nextRefresh) {
          refresh();
          nextRefresh = now + 100;
        }
        for (const panel of panels.values()) panel.tick();
        scene.updateMatrixWorld(true);
        pointers?.update(frame, now);
        renderer.render(scene, camera);
      } catch (cause) {
        report(`VR 呈现失败：${String(cause)}`);
        options.exit();
        dispose();
      }
    });
    save();
    return { dispose };
  } catch (cause) {
    dispose();
    throw cause;
  }
}
