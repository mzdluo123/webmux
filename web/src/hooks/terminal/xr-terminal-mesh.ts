import type { Terminal } from "@xterm/xterm";
import {
  type CanvasTexture,
  Mesh,
  MeshBasicMaterial,
  PlaneGeometry,
  type Group,
} from "three";
import {
  createXrTerminalCanvas,
  type XrTerminalCanvas,
} from "./xr-terminal-canvas";
import { createXrTexture } from "./xr-surface";

/** Owns only XR graphics; disposing never touches the shared emulator or socket. */
export class XrTerminalMesh {
  private terminal: Terminal | undefined;
  private canvas: XrTerminalCanvas | undefined;
  private mesh: Mesh<PlaneGeometry, MeshBasicMaterial> | undefined;
  private texture: CanvasTexture | undefined;
  private textureWidth = 0;
  private textureHeight = 0;

  constructor(private readonly parent: Group) {}

  sync(terminal: Terminal | undefined, size: readonly [number, number]) {
    if (terminal === this.terminal) return;
    this.dispose();
    this.terminal = terminal;
    if (!terminal) return;
    this.canvas = createXrTerminalCanvas(terminal);
    this.canvas.update();
    this.texture = createXrTexture(this.canvas.canvas);
    this.textureWidth = this.canvas.canvas.width;
    this.textureHeight = this.canvas.canvas.height;
    this.mesh = new Mesh(
      new PlaneGeometry(1, 1),
      new MeshBasicMaterial({ map: this.texture, toneMapped: false }),
    );
    this.mesh.position.set(-0.015, -0.005, 0);
    this.mesh.renderOrder = 1;
    this.parent.add(this.mesh);
    this.fit(size);
  }

  set visible(value: boolean) {
    if (this.mesh) this.mesh.visible = value;
  }

  update(size: readonly [number, number]) {
    if (!this.canvas?.update() || !this.texture || !this.mesh) return;
    const image = this.canvas.canvas;
    if (
      image.width !== this.textureWidth ||
      image.height !== this.textureHeight
    ) {
      // Three's texture storage is immutable after its first GPU upload.
      const previous = this.texture;
      this.texture = createXrTexture(image);
      this.textureWidth = image.width;
      this.textureHeight = image.height;
      this.mesh.material.map = this.texture;
      previous.dispose();
    } else this.texture.needsUpdate = true;
    this.fit(size);
  }

  fit([width, height]: readonly [number, number]) {
    const canvas = this.canvas?.canvas;
    if (!canvas?.height || !this.mesh) return;
    const aspect = canvas.width / canvas.height;
    const fittedWidth = Math.min(width * 0.94, height * 0.69 * aspect);
    const fittedHeight = fittedWidth / aspect;
    this.mesh.scale.set(fittedWidth / width, fittedHeight / height, 1);
  }

  dispose() {
    this.canvas?.dispose();
    this.mesh?.removeFromParent();
    this.mesh?.geometry.dispose();
    this.mesh?.material.dispose();
    this.texture?.dispose();
    this.terminal = undefined;
    this.canvas = undefined;
    this.mesh = undefined;
    this.texture = undefined;
  }
}
