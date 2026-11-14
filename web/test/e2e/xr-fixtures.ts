import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { Page } from "@playwright/test";
import type * as Iwer from "iwer";
import type { XRDevice } from "iwer";
import type { XrPanelLayout } from "../../src/domains/terminal/xr-layout";
import { expect, test as base } from "./fixtures";

type PaintedText = { text: string; x: number; y: number };
export type PaintedSurface = {
  id: number;
  width: number;
  height: number;
  text: PaintedText[];
};

type XrProbe = {
  device: XRDevice;
  ended: boolean;
  session?: XRSession;
  surfaces: Map<HTMLCanvasElement, PaintedSurface>;
  uploaded: Set<HTMLCanvasElement>;
  draws: number;
};

declare global {
  interface Window {
    IWER: typeof Iwer;
    xrE2E: XrProbe;
  }
}

/** Only native Canvas/WebGL output and public WebXR/IWER APIs are observed. */
async function installIwer(page: Page) {
  const entry = fileURLToPath(import.meta.resolve("iwer"));
  const bundle = await readFile(
    join(dirname(entry), "../build/iwer.min.js"),
    "utf8",
  );
  function initialize() {
    const device = new window.IWER.XRDevice(window.IWER.metaQuest3, {
      stereoEnabled: false,
    });
    device.installRuntime({ forceInstall: true });
    device.position.set(0, 1.6, 0);
    device.controllers.left!.connected = false;
    const probe: XrProbe = {
      device,
      surfaces: new Map(),
      uploaded: new Set(),
      ended: false,
      draws: 0,
    };
    window.xrE2E = probe;
    const request = navigator.xr!.requestSession.bind(navigator.xr);
    navigator.xr!.requestSession = async (...args) => {
      const session = await request(...args);
      probe.session = session;
      probe.ended = false;
      session.addEventListener("end", () => {
        probe.ended = true;
      });
      return session;
    };
    let sequence = 0;
    const fillRect = CanvasRenderingContext2D.prototype.fillRect;
    CanvasRenderingContext2D.prototype.fillRect = function (x, y, w, h) {
      const canvas = this.canvas;
      if (x === 0 && y === 0 && w === canvas.width && h === canvas.height) {
        const previous = probe.surfaces.get(canvas);
        probe.surfaces.set(canvas, {
          id: previous?.id ?? ++sequence,
          width: canvas.width,
          height: canvas.height,
          text: [],
        });
        probe.uploaded.delete(canvas);
      }
      return fillRect.call(this, x, y, w, h);
    };
    const fillText = CanvasRenderingContext2D.prototype.fillText;
    CanvasRenderingContext2D.prototype.fillText = function (text, x, y, max) {
      probe.surfaces.get(this.canvas)?.text.push({ text, x, y });
      if (max === undefined) return fillText.call(this, text, x, y);
      return fillText.call(this, text, x, y, max);
    };
    // Record actual texture uploads, not merely allocated/offscreen canvases.
    for (const prototype of [
      WebGLRenderingContext.prototype,
      WebGL2RenderingContext.prototype,
    ]) {
      for (const name of ["texImage2D", "texSubImage2D"] as const) {
        const upload = prototype[name];
        Object.defineProperty(prototype, name, {
          configurable: true,
          value: function (
            this: WebGLRenderingContext | WebGL2RenderingContext,
            ...args: unknown[]
          ) {
            for (const value of args)
              if (value instanceof HTMLCanvasElement) probe.uploaded.add(value);
            return Reflect.apply(upload, this, args);
          },
        });
      }
      const draw = prototype.drawElements;
      prototype.drawElements = function (...args) {
        probe.draws++;
        return draw.apply(this, args);
      };
    }
  }
  // Playwright does not promise ordering between separate init scripts.
  await page.addInitScript({
    content: `${bundle}\n;(${initialize.toString()})();`,
  });
}

export class XrBrowser {
  private readonly surfaceIds = new Map<string, number>();
  constructor(readonly page: Page) {}

  async frames(count = 3) {
    await this.page.evaluate(async (count) => {
      for (let index = 0; index < count; index++) {
        if (window.xrE2E.ended) return;
        const next = Promise.withResolvers<void>();
        const session = window.xrE2E.session!;
        session.addEventListener("end", next.resolve, { once: true });
        session.requestAnimationFrame(() => next.resolve());
        await next.promise;
        session.removeEventListener("end", next.resolve);
      }
    }, count);
  }

  async enter() {
    this.surfaceIds.clear();
    const before = await this.page.evaluate(() => window.xrE2E.draws);
    await this.page
      .getByRole("button", { name: "进入 VR", exact: true })
      .click();
    await expect
      .poll(() =>
        this.page.evaluate(
          (before) =>
            !window.xrE2E.ended &&
            !!window.xrE2E.session &&
            window.xrE2E.draws > before,
          before,
        ),
      )
      .toBe(true);
    await this.frames();
  }

  async surfaces(): Promise<PaintedSurface[]> {
    return this.page.evaluate(() =>
      [...window.xrE2E.surfaces].flatMap(([canvas, surface]) =>
        canvas.width && window.xrE2E.uploaded.has(canvas) ? [surface] : [],
      ),
    );
  }

  async surface(id: number) {
    return (await this.surfaces()).find((surface) => surface.id === id);
  }

  async text(id: number) {
    return (
      (await this.surface(id))?.text.map((line) => line.text).join("\n") ?? ""
    );
  }

  async findSurface(title: string) {
    let found: PaintedSurface | undefined;
    await expect
      .poll(
        async () => {
          const surfaces = await this.surfaces();
          found =
            surfaces.find((surface) =>
              surface.text.some(
                (line) => line.y === 33 && line.text.includes(title),
              ),
            ) ??
            surfaces.find(
              (surface) => surface.id === this.surfaceIds.get(title),
            );
          return !!found;
        },
        { message: `XR texture showing ${title}` },
      )
      .toBe(true);
    // Confirmations replace the title but keep the same public canvas texture.
    // Only reuse an identity while that canvas still has current uploaded paint.
    this.surfaceIds.set(title, found!.id);
    return found!;
  }

  async layouts(): Promise<XrPanelLayout[]> {
    return this.page.evaluate(
      () =>
        JSON.parse(localStorage.getItem("webmux.xr-workspace-layout") ?? "{}")
          .panels ?? [],
    );
  }

  async point(
    position: [number, number, number],
    rayOriginZ = position[2] + 0.03,
  ) {
    await this.page.evaluate(
      ([x, y, z]) => {
        const controller = window.xrE2E.device.controllers.right!;
        controller.position.set(x, y + 1.6, z);
        controller.quaternion.set(0, 0, 0, 1);
      },
      [position[0], position[1], rayOriginZ],
    );
    await this.frames();
  }

  async trigger(value: number) {
    await this.page.evaluate((value) => {
      window.xrE2E.device.controllers.right!.updateButtonValue(
        "trigger",
        value,
      );
    }, value);
    await this.frames();
  }

  async hit(
    position: [number, number, number],
    size: [number, number],
    resolution: [number, number],
    x: number,
    y: number,
  ) {
    await this.point([
      position[0] + (x / resolution[0] - 0.5) * size[0],
      position[1] + (0.5 - y / resolution[1]) * size[1],
      position[2],
    ]);
    await this.trigger(1);
    await this.trigger(0);
  }

  async panel(layout: XrPanelLayout, x: number, y: number) {
    await this.hit(layout.position, layout.size, [1024, 768], x, y);
  }

  async menu(label: string) {
    let line: PaintedText | undefined;
    await expect
      .poll(
        async () => {
          const menu = (await this.surfaces()).find(
            (surface) => surface.width === 800,
          );
          line = menu?.text.find(
            (item) => item.text === label || item.text.includes(label),
          );
          return !!line;
        },
        { message: `XR menu ${label}` },
      )
      .toBe(true);
    await this.hit(
      [-1.35, 0, -1.75],
      [0.8, 1.45],
      [800, 1450],
      line!.x + 20,
      line!.y,
    );
  }

  async unobserve(name: string) {
    const menu = (await this.surfaces()).find(
      (surface) => surface.width === 800,
    );
    const row = menu?.text.find(
      (line) => line.y < 450 && line.text.includes(name),
    );
    expect(row, `Observed session ${name}`).toBeTruthy();
    await this.hit([-1.35, 0, -1.75], [0.8, 1.45], [800, 1450], 690, row!.y);
  }

  async key(value: string) {
    let line: PaintedText | undefined;
    await expect
      .poll(
        async () => {
          const keyboard = (await this.surfaces()).find(
            (surface) => surface.width === 1200,
          );
          line = keyboard?.text.find((item) => item.text === value);
          return keyboard?.text.map((item) => item.text) ?? [];
        },
        { message: `Uploaded VR keyboard showing key ${value}` },
      )
      .toContain(value);
    await this.hit(
      [0, -0.95, -1.7],
      [1.5, 0.675],
      [1200, 540],
      line!.x + 20,
      line!.y,
    );
  }

  async scroll(layout: XrPanelLayout, direction: -1 | 1) {
    await this.point([
      layout.position[0],
      layout.position[1],
      layout.position[2],
    ]);
    await this.page.evaluate((direction) => {
      window.xrE2E.device.controllers.right!.updateAxes(
        "thumbstick",
        0,
        direction,
      );
    }, direction);
    // Scroll throttling is production time-based, so retain the real XR frame loop.
    await this.page.waitForTimeout(160);
    await this.page.evaluate(() => {
      window.xrE2E.device.controllers.right!.updateAxes("thumbstick", 0, 0);
    });
    await this.frames();
  }
}

export const test = base.extend<{ xr: XrBrowser }>({
  xr: async ({ page }, use) => {
    await installIwer(page);
    await use(new XrBrowser(page));
  },
});
export { expect };

/** Drop real chat transports and hold their resume handshakes while agents run. */
export async function holdChatReattach(page: Page) {
  const gate = Promise.withResolvers<void>();
  let held = false;
  const sockets = new Set<() => Promise<void>>();
  await page.routeWebSocket(/\/ws\/s\/[^/]+\/chat(?:\?|$)/, async (socket) => {
    if (held) await gate.promise;
    socket.connectToServer();
    sockets.add(() => socket.close());
  });
  return {
    async drop() {
      expect(sockets.size, "Live isolated ACP transports").toBeGreaterThan(0);
      held = true;
      await Promise.all([...sockets].map((close) => close()));
      sockets.clear();
    },
    release() {
      held = false;
      gate.resolve();
    },
  };
}
