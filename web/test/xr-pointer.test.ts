import {
  Group,
  Matrix4,
  Mesh,
  MeshBasicMaterial,
  PlaneGeometry,
  Scene,
} from "three";
import { describe, expect, test, vi } from "vite-plus/test";
import { defaultXrPanel } from "../src/domains/terminal/xr-layout";
import { createXrPointers } from "../src/hooks/terminal/xr-pointer";
import type { XrPanel } from "../src/hooks/terminal/xr-panel";
import type { XrRegion, XrSurface } from "../src/hooks/terminal/xr-surface";

function fixture(mode?: XrRegion["mode"]) {
  const source = {
    targetRaySpace: {},
    targetRayMode: "tracked-pointer",
  } as XRInputSource;
  const events = Object.assign(new EventTarget(), {
    inputSources: [source],
    visibilityState: "visible",
  });
  const session = events as unknown as XRSession;
  const scene = new Scene();
  const mesh = new Mesh(new PlaneGeometry(1, 1), new MeshBasicMaterial());
  const group = new Group();
  group.add(mesh);
  group.position.z = -2;
  group.scale.set(2, 2, 1);
  scene.add(group);
  const run = vi.fn();
  const changed = vi.fn();
  const scroll = vi.fn();
  const region: XrRegion = {
    id: "target",
    x: 0,
    y: 0,
    width: 100,
    height: 100,
    mode,
    run,
  };
  const surface = {
    mesh,
    group,
    hit: () => region,
    setHover: vi.fn(),
  } as unknown as XrSurface;
  const layout = defaultXrPanel("terminal:one", "one", "terminal", 1);
  layout.position = [0, 0, -2];
  layout.size = [2, 2];
  const panel = {
    layout,
    scroll,
    applyLayout: () => {
      group.position.fromArray(layout.position);
      group.scale.set(...layout.size, 1);
    },
  } as unknown as XrPanel;
  const surfaces = [surface];
  const panels = new Map([[surface, panel]]);
  let x = 0;
  let y = 0;
  const matrix = new Matrix4();
  const frame = {
    getPose: () => ({
      transform: { matrix: matrix.makeTranslation(x, y, 0).elements },
    }),
  } as unknown as XRFrame;
  const pointers = createXrPointers(
    session,
    scene,
    {} as XRReferenceSpace,
    () => surfaces,
    (target) => panels.get(target),
    changed,
  );
  function dispatch(type: string, inputSource = source) {
    events.dispatchEvent(
      Object.assign(new Event(type), { inputSource, frame }),
    );
  }
  function update(nextX = 0, nextY = 0) {
    x = nextX;
    y = nextY;
    scene.updateMatrixWorld(true);
    pointers.update(frame, 1000);
  }
  return {
    events,
    source,
    scene,
    group,
    surface,
    surfaces,
    panels,
    frame,
    run,
    changed,
    scroll,
    layout,
    dispatch,
    update,
    dispose() {
      pointers.dispose();
      mesh.geometry.dispose();
      mesh.material.dispose();
    },
  };
}

describe("XR controller capture boundaries", () => {
  test.each(["tracked-pointer", "gaze", "screen"] as const)(
    "%s selects once on matching release, never again on duplicate release",
    (targetRayMode) => {
      const value = fixture();
      Object.assign(value.source, { targetRayMode });
      value.update();
      value.dispatch("selectstart");
      expect(value.run).not.toHaveBeenCalled();
      value.dispatch("selectend");
      value.dispatch("selectend");
      expect(value.run).toHaveBeenCalledTimes(1);
      value.dispose();
      expect(value.scene.children).toEqual([value.group]);
    },
  );

  test("release outside the original surface cancels the action", () => {
    const value = fixture();
    value.update();
    value.dispatch("selectstart");
    value.update(4);
    value.dispatch("selectend");
    expect(value.run).not.toHaveBeenCalled();
    value.dispose();
  });

  test("disconnect ends a drag at its last position and does not transfer to a new source", () => {
    const value = fixture("move");
    value.update();
    value.dispatch("selectstart");
    value.update(0.5);
    expect(value.layout.position[0]).toBeCloseTo(0.5);
    value.events.dispatchEvent(
      Object.assign(new Event("inputsourceschange"), {
        removed: [value.source],
        added: [],
      }),
    );
    const replacement = {
      targetRaySpace: {},
      targetRayMode: "screen",
    } as XRInputSource;
    value.events.inputSources = [replacement];
    value.update(-0.5);
    value.dispatch("selectend", replacement);
    expect(value.layout.position[0]).toBeCloseTo(0.5);
    expect(value.changed).toHaveBeenCalledTimes(1);
    expect(value.run).not.toHaveBeenCalled();
    value.dispose();
  });

  test("drag-scroll stops when the ray leaves the surface", () => {
    const value = fixture("scroll");
    value.update();
    value.dispatch("selectstart");
    value.update(0, 0.4);
    expect(value.scroll).toHaveBeenCalledTimes(1);
    value.update(4, 0.8);
    expect(value.scroll).toHaveBeenCalledTimes(1);
    value.dispose();
  });

  test("visibility loss cancels a pressed key before release", () => {
    const value = fixture();
    value.update();
    value.dispatch("selectstart");
    value.events.visibilityState = "hidden";
    value.events.dispatchEvent(new Event("visibilitychange"));
    value.dispatch("selectend");
    expect(value.run).not.toHaveBeenCalled();
    value.dispose();
  });
});

test.each([false, true])(
  "coplanar selection and dragging follow paint order but retain real depth (recorded controller: %s)",
  (recorded) => {
    const value = fixture("move");
    let chosen = "";
    const region = (id: string): XrRegion => ({
      id,
      x: 0,
      y: 0,
      width: 100,
      height: 100,
      mode: "move",
      run: () => {
        chosen = id;
      },
    });
    const lower = region("lower");
    const upper = region("upper");
    value.surface.hit = () => lower;
    const mesh = new Mesh(new PlaneGeometry(1, 1), new MeshBasicMaterial());
    const group = new Group();
    group.add(mesh);
    const layout = defaultXrPanel("terminal:upper", "upper", "terminal", 2);
    layout.position = [0, 0, -2];
    if (recorded) {
      value.layout.position = [0, 0, -2.4];
      value.layout.size = [1.6, 1.1];
      value.panels.get(value.surface)!.applyLayout();
      layout.position = [8.827371811293006e-8, -1.0248077275676337e-7, -2.4];
    }
    layout.size = recorded ? [1.6, 1.1] : [2.2, 2.4];
    const panel = {
      layout,
      scroll() {},
      applyLayout() {
        group.position.fromArray(layout.position);
        group.scale.set(...layout.size, 1);
      },
    } as unknown as XrPanel;
    panel.applyLayout();
    const surface = {
      mesh,
      group,
      hit: () => upper,
      setHover() {},
    } as unknown as XrSurface;
    value.scene.add(group);
    value.surfaces.push(surface);
    value.panels.set(surface, panel);
    // Standard XRFrame.getPose matrix recorded after dragging overlapping panels.
    const pose = recorded
      ? new Matrix4().fromArray([
          0.9896631836891174, 0.02086467295885086, -0.141885444521904, 0,
          0.02086467295885086, 0.9578850865364075, 0.2863927185535431, 0,
          0.141885444521904, -0.2863927185535431, 0.9475482702255249, 0,
          0.30000001192092896, -0.30000007152557373, 0, 1,
        ])
      : new Matrix4().makeRotationY(0.13).setPosition(0.15, 0.2, 0);
    const lowerPosition = [...value.layout.position];
    Object.assign(value.frame, {
      getPose: () => ({ transform: { matrix: pose.elements } }),
    });
    try {
      value.update();
      value.dispatch("selectstart");
      value.dispatch("selectend");
      expect(chosen).toBe("upper");
      value.dispatch("selectstart");
      if (recorded) pose.setPosition(0.65, -0.05, 0);
      else pose.setPosition(0.5, 0.45, 0);
      value.update();
      value.dispatch("selectend");
      expect(layout.position[0]).toBeCloseTo(0.35);
      expect(layout.position[1]).toBeCloseTo(0.25);
      expect(value.layout.position).toEqual(lowerPosition);
      value.layout.position[2] = -1.5;
      value.panels.get(value.surface)!.applyLayout();
      value.update();
      chosen = "";
      value.dispatch("selectstart");
      value.dispatch("selectend");
      expect(chosen).toBe("lower");
    } finally {
      group.removeFromParent();
      mesh.geometry.dispose();
      mesh.material.dispose();
      value.dispose();
    }
  },
);
