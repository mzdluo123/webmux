import { Color, PerspectiveCamera, Scene, WebGLRenderer } from "three";
import { z } from "zod";
import {
  decodeXrLayout,
  encodeXrLayout,
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
import { createXrViewState } from "./xr-view-state";

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
  const preference = deviceObjectPreference(
    "xr-workspace-layout",
    z.unknown().transform((value) => {
      const panels = decodeXrLayout(value);
      return panels === undefined ? undefined : encodeXrLayout(panels);
    }),
    () => undefined,
  );
  let disposed = false;
  let nextRefresh = 0;
  const panels = new Map<string, XrPanel>();
  const views = createXrViewState(
    options,
    preference.value?.panels,
    () => {
      save();
      nextRefresh = 0;
    },
    report,
  );
  resources.push(() => {
    views.dispose();
    for (const panel of panels.values()) panel.dispose();
    panels.clear();
  });
  function save() {
    if (
      preference.value === undefined &&
      views.layouts.length === 0 &&
      views.pending.size
    )
      return;
    preference.set(encodeXrLayout(views.layouts));
  }
  const keyboard = createXrKeyboard(views.selection);
  resources.push(() => keyboard.dispose());
  const menu = createXrMenu(
    options,
    views,
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
    const { sources, texts } = views.refresh();
    const currentIds = new Set(views.layouts.map((layout) => layout.id));
    for (const [id, panel] of panels) {
      if (!currentIds.has(id)) {
        panel.dispose();
        panels.delete(id);
      }
    }
    for (const layout of views.layouts) {
      const source = sources.find((item) => item.id === layout.sessionId);
      const text = texts
        .get(layout.sessionId)
        ?.find(
          (item) => item.kind === layout.kind && item.id === layout.sourceId,
        );
      let panel = panels.get(layout.id);
      if (!panel) {
        panel = new XrPanel(
          layout,
          views.select,
          () => {
            views.reconcile();
            save();
            nextRefresh = 0;
          },
          () => {
            void options.reveal(layout.sessionId);
          },
          report,
          views.split,
        );
        panels.set(layout.id, panel);
        scene.add(panel.surface.group);
      }
      if (!source || (layout.kind !== "terminal" && !text)) {
        panel.detach();
        continue;
      }
      panel.applyLayout();
      panel.update(source, text, views.selectedViewId === layout.id);
    }
    surfaces.length = 2;
    for (const panel of panels.values()) surfaces.push(panel.surface);
    orderXrSurfaces(surfaces);
    menu.update(texts);
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
    void views.initialize();
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
