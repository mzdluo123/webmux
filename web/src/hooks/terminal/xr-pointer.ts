import {
  BufferGeometry,
  Float32BufferAttribute,
  Line,
  Group,
  LineBasicMaterial,
  Matrix4,
  Raycaster,
  Vector3,
  type Intersection,
  type Object3D,
  type Scene,
} from "three";
import { boundXrPanel, XR_PANEL_LIMIT } from "../../domains/terminal/xr-layout";
import type { XrPanel } from "./xr-panel";
import type { XrRegion, XrSurface } from "./xr-surface";

interface Hit {
  surface: XrSurface;
  region: XrRegion;
  point: Vector3;
  distance: number;
}
interface Capture {
  hit: Hit;
  start: Vector3;
  position?: [number, number, number];
  size?: [number, number];
  moved: boolean;
  scrollY: number;
}
interface Pointer {
  ray: Raycaster;
  line: Line<BufferGeometry, LineBasicMaterial>;
  capture?: Capture;
  hit?: Hit;
  lastScroll: number;
}

/** Input identity, not controller index, owns every capture. */
export function createXrPointers(
  session: XRSession,
  scene: Scene,
  reference: XRReferenceSpace,
  surfaces: () => readonly XrSurface[],
  panelFor: (surface: XrSurface) => XrPanel | undefined,
  changed: () => void,
) {
  const pointers = new Map<XRInputSource, Pointer>();
  const matrix = new Matrix4();
  const direction = new Vector3();
  const point = new Vector3();
  const intersections: Intersection<Object3D>[] = [];
  const geometry = new BufferGeometry();
  geometry.setAttribute(
    "position",
    new Float32BufferAttribute([0, 0, 0, 0, 0, -1], 3),
  );
  const material = new LineBasicMaterial({
    color: "#83d3a0",
    depthTest: false,
  });
  const rays = new Group();
  rays.renderOrder = XR_PANEL_LIMIT + 2;
  scene.add(rays);

  function getPointer(source: XRInputSource) {
    let pointer = pointers.get(source);
    if (!pointer) {
      const line = new Line(geometry, material);
      rays.add(line);
      pointer = { ray: new Raycaster(), line, lastScroll: 0 };
      pointers.set(source, pointer);
    }
    return pointer;
  }

  function aim(source: XRInputSource, frame: XRFrame) {
    const pointer = getPointer(source);
    const pose = frame.getPose(source.targetRaySpace, reference);
    pointer.line.visible = !!pose;
    if (!pose) {
      pointer.hit = undefined;
      return pointer;
    }
    matrix.fromArray(pose.transform.matrix);
    pointer.line.matrixAutoUpdate = false;
    pointer.line.matrix.copy(matrix);
    pointer.ray.ray.origin.setFromMatrixPosition(matrix);
    direction.set(0, 0, -1).transformDirection(matrix);
    pointer.ray.ray.direction.copy(direction);
    pointer.hit = undefined;
    let nearest = Infinity;
    for (const surface of surfaces()) {
      if (!surface.group.visible) continue;
      intersections.length = 0;
      pointer.ray.intersectObject(surface.mesh, false, intersections);
      const hit = intersections[0];
      // Coplanar triangle transforms can differ by a few arithmetic ULPs.
      if (
        !hit?.uv ||
        hit.distance - nearest > Number.EPSILON * 8 * Math.max(1, hit.distance)
      )
        continue;
      const region = surface.hit(hit.uv.x, hit.uv.y);
      nearest = hit.distance;
      pointer.hit = region
        ? { surface, region, point: hit.point.clone(), distance: hit.distance }
        : undefined;
    }
    pointer.line.scale.z = Number.isFinite(nearest) ? nearest : 5;
    // matrixAutoUpdate is off, so scale the direction column without moving the origin.
    pointer.line.matrix.elements[8] =
      matrix.elements[8]! * pointer.line.scale.z;
    pointer.line.matrix.elements[9] =
      matrix.elements[9]! * pointer.line.scale.z;
    pointer.line.matrix.elements[10] =
      matrix.elements[10]! * pointer.line.scale.z;
    pointer.line.matrixWorldNeedsUpdate = true;
    return pointer;
  }

  function down(event: XRInputSourceEvent) {
    if (session.visibilityState !== "visible") return;
    scene.updateMatrixWorld(true);
    const pointer = aim(event.inputSource, event.frame);
    const hit = pointer.hit;
    if (!hit || pointer.capture) return;
    // One hand cannot steal an object currently held by another.
    if (
      [...pointers.values()].some(
        (other) => other.capture?.hit.surface === hit.surface,
      )
    )
      return;
    const panel = panelFor(hit.surface);
    pointer.capture = {
      hit,
      start: hit.point.clone(),
      position: panel ? [...panel.layout.position] : undefined,
      size: panel ? [...panel.layout.size] : undefined,
      moved: false,
      scrollY: hit.point.y,
    };
  }

  function up(event: XRInputSourceEvent) {
    const pointer = pointers.get(event.inputSource);
    const capture = pointer?.capture;
    if (!pointer || !capture) return;
    pointer.capture = undefined;
    scene.updateMatrixWorld(true);
    aim(event.inputSource, event.frame);
    if (capture.moved) changed();
    else if (
      pointer.hit?.surface === capture.hit.surface &&
      pointer.hit.region.id === capture.hit.region.id
    )
      pointer.hit.region.run?.();
  }

  function cancel(source: XRInputSource) {
    const pointer = pointers.get(source);
    if (pointer?.capture?.moved) changed();
    pointer?.line.removeFromParent();
    pointers.delete(source);
  }

  function sourcesChanged(event: XRInputSourcesChangeEvent) {
    for (const source of event.removed) cancel(source);
  }
  function visibilityChanged() {
    if (session.visibilityState !== "visible")
      for (const pointer of pointers.values()) {
        if (pointer.capture?.moved) changed();
        pointer.capture = undefined;
      }
  }
  session.addEventListener("selectstart", down);
  session.addEventListener("selectend", up);
  session.addEventListener("inputsourceschange", sourcesChanged);
  session.addEventListener("visibilitychange", visibilityChanged);

  return {
    update(frame: XRFrame, now: number) {
      for (const source of session.inputSources) {
        const pointer = aim(source, frame);
        const capture = pointer.capture;
        const panel = capture && panelFor(capture.hit.surface);
        if (capture && panel && pointer.line.visible) {
          // Fixed panel plane prevents movement of the target from feeding back into the drag.
          const dz = pointer.ray.ray.direction.z;
          const distance =
            Math.abs(dz) > 0.001
              ? (capture.start.z - pointer.ray.ray.origin.z) / dz
              : -1;
          if (distance > 0) {
            pointer.ray.ray.at(distance, point);
            const dx = point.x - capture.start.x;
            const dy = point.y - capture.start.y;
            if (Math.hypot(dx, dy) > 0.012) capture.moved = true;
            if (capture.hit.region.mode === "move" && capture.position) {
              panel.layout.position[0] = capture.position[0] + dx;
              panel.layout.position[1] = capture.position[1] + dy;
            } else if (capture.hit.region.mode === "resize" && capture.size) {
              panel.layout.size[0] = capture.size[0] + dx * 2;
              panel.layout.size[1] = capture.size[1] - dy * 2;
            } else if (
              capture.hit.region.mode === "scroll" &&
              pointer.hit?.surface === capture.hit.surface
            ) {
              const lines = Math.trunc(
                (point.y - capture.scrollY) / (panel.layout.size[1] / 28),
              );
              if (lines) {
                panel.scroll(lines);
                capture.scrollY = point.y;
              }
            }
            boundXrPanel(panel.layout);
            panel.applyLayout();
          }
        }
        const axis = source.gamepad?.axes;
        const scroll = axis && (axis.length >= 4 ? axis[3] : axis[1]);
        if (
          !capture &&
          pointer.hit &&
          scroll &&
          Math.abs(scroll) > 0.25 &&
          now - pointer.lastScroll > 100
        ) {
          panelFor(pointer.hit.surface)?.scroll(scroll > 0 ? 3 : -3);
          pointer.lastScroll = now;
        }
      }
      for (const surface of surfaces()) {
        const hover = new Set<string>();
        for (const pointer of pointers.values())
          if (pointer.hit?.surface === surface)
            hover.add(pointer.hit.region.id);
        surface.setHover(hover);
      }
    },
    dispose() {
      session.removeEventListener("selectstart", down);
      session.removeEventListener("selectend", up);
      session.removeEventListener("inputsourceschange", sourcesChanged);
      session.removeEventListener("visibilitychange", visibilityChanged);
      for (const source of pointers.keys()) cancel(source);
      rays.removeFromParent();
      geometry.dispose();
      material.dispose();
    },
  };
}
