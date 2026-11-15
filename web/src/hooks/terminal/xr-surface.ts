import {
  CanvasTexture,
  Group,
  LinearFilter,
  Mesh,
  MeshBasicMaterial,
  PlaneGeometry,
  SRGBColorSpace,
} from "three";

/** Shared color/filter contract for UI and public-buffer terminal canvases. */
export function createXrTexture(canvas: HTMLCanvasElement): CanvasTexture {
  const texture = new CanvasTexture(canvas);
  texture.colorSpace = SRGBColorSpace;
  texture.minFilter = LinearFilter;
  texture.generateMipmaps = false;
  return texture;
}

export interface XrRegion {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  label?: string;
  selected?: boolean;
  disabled?: boolean;
  mode?: "move" | "resize" | "scroll";
  run?(): void;
}

/** Equal-depth surfaces paint and receive ray input in this shared order. */
export function orderXrSurfaces(surfaces: readonly XrSurface[]) {
  for (let index = 0; index < surfaces.length; index++)
    surfaces[index]!.group.renderOrder = index;
}

/** One bounded texture and hit map per spatial surface. No DOM capture. */
export class XrSurface {
  readonly group = new Group();
  readonly canvas = document.createElement("canvas");
  readonly context: CanvasRenderingContext2D;
  readonly texture: CanvasTexture;
  readonly mesh: Mesh<PlaneGeometry, MeshBasicMaterial>;
  regions: XrRegion[] = [];
  private hover = new Set<string>();
  private paintContent: (() => void) | undefined;

  constructor(
    readonly width = 1024,
    readonly height = 768,
  ) {
    this.canvas.width = width;
    this.canvas.height = height;
    const context = this.canvas.getContext("2d");
    if (!context) throw new Error("VR 面板无法创建 Canvas 2D 上下文");
    this.context = context;
    this.texture = createXrTexture(this.canvas);
    this.mesh = new Mesh(
      new PlaneGeometry(1, 1),
      new MeshBasicMaterial({ map: this.texture, toneMapped: false }),
    );
    this.group.add(this.mesh);
  }

  setSize(width: number, height: number) {
    this.group.scale.set(width, height, 1);
  }

  draw(paint: () => void) {
    this.paintContent = paint;
    this.repaint();
  }

  private repaint() {
    const ctx = this.context;
    ctx.fillStyle = "#101412";
    ctx.fillRect(0, 0, this.width, this.height);
    this.paintContent?.();
    for (const region of this.regions) {
      if (!region.label) continue;
      ctx.fillStyle = region.disabled
        ? "#222723"
        : this.hover.has(region.id)
          ? "#294b36"
          : "#19221c";
      ctx.fillRect(
        region.x + 2,
        region.y + 2,
        region.width - 4,
        region.height - 4,
      );
      ctx.strokeStyle =
        region.selected || this.hover.has(region.id) ? "#83d3a0" : "#63746a";
      ctx.lineWidth = region.selected ? 3 : 1;
      ctx.strokeRect(
        region.x + 3,
        region.y + 3,
        region.width - 6,
        region.height - 6,
      );
      ctx.fillStyle = region.disabled ? "#89928c" : "#edf4ef";
      ctx.font = "24px system-ui, sans-serif";
      ctx.textBaseline = "middle";
      ctx.save();
      ctx.beginPath();
      ctx.rect(region.x + 8, region.y, region.width - 16, region.height);
      ctx.clip();
      ctx.fillText(region.label, region.x + 12, region.y + region.height / 2);
      ctx.restore();
    }
    this.texture.needsUpdate = true;
  }

  setHover(ids: Set<string>) {
    if (
      ids.size === this.hover.size &&
      [...ids].every((id) => this.hover.has(id))
    )
      return;
    this.hover = ids;
    this.repaint();
  }

  hit(u: number, v: number): XrRegion | undefined {
    const x = u * this.width;
    const y = (1 - v) * this.height;
    for (let index = this.regions.length - 1; index >= 0; index--) {
      const region = this.regions[index]!;
      if (
        x >= region.x &&
        y >= region.y &&
        x <= region.x + region.width &&
        y <= region.y + region.height
      )
        return region.disabled ? undefined : region;
    }
    return undefined;
  }

  text(
    text: string,
    x: number,
    y: number,
    color = "#edf4ef",
    font = "24px monospace",
  ) {
    this.context.font = font;
    this.context.textBaseline = "top";
    this.context.fillStyle = color;
    this.context.fillText(text, x, y);
  }

  dispose() {
    this.group.visible = false;
    this.group.removeFromParent();
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
    this.texture.dispose();
    this.canvas.width = this.canvas.height = 0;
    this.regions = [];
    this.paintContent = undefined;
  }
}
