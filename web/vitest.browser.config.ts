import { availableParallelism } from "node:os";
import { mergeConfig } from "vite";
import { defineConfig } from "vite-plus";
import {
  defineBrowserCommand,
  playwright,
} from "vite-plus/test/browser-playwright";
import viteConfig from "./vite.config";
import { sharedCoverage } from "./vitest.config";

// Reuse the production compiler, Tailwind and aliases, but never its API proxy.
const { server: _applicationServer, ...sharedConfig } = viteConfig;

// nativeTouchGesture accepts CSS-pixel points in the test iframe's viewport.
// The first point starts one real Chromium finger, subsequent points move it,
// and the command releases it (or cancels when requested). CDP coordinates are
// translated to the top-level page; emulation and the session are always
// released, including on failure.
const nativeTouchGesture = defineBrowserCommand(
  async (
    { page, frame },
    points: { x: number; y: number }[],
    finish: "end" | "cancel" = "end",
    holdMilliseconds = 0,
  ) => {
    const testFrame = await frame();
    const bounds = await (await testFrame.frameElement()).boundingBox();
    if (!bounds || !points.length)
      throw new Error("Missing touch gesture target");
    const viewport = await testFrame.evaluate(() => ({
      width: innerWidth,
      height: innerHeight,
    }));
    const cdp = await page.context().newCDPSession(page);
    const delivered = await testFrame.evaluateHandle(() => {
      const point = { x: NaN, y: NaN, dispose: () => {} };
      const receive = (event: PointerEvent) => {
        if (event.pointerType !== "touch") return;
        point.x = event.clientX;
        point.y = event.clientY;
      };
      document.addEventListener("pointerdown", receive, true);
      document.addEventListener("pointermove", receive, true);
      point.dispose = () => {
        document.removeEventListener("pointerdown", receive, true);
        document.removeEventListener("pointermove", receive, true);
      };
      return point;
    });
    try {
      await cdp.send("Emulation.setTouchEmulationEnabled", {
        enabled: true,
        maxTouchPoints: 2,
      });
      for (const [index, point] of points.entries()) {
        await cdp.send("Input.dispatchTouchEvent", {
          type: index === 0 ? "touchStart" : "touchMove",
          touchPoints: [
            {
              x: bounds.x + (point.x * bounds.width) / viewport.width,
              y: bounds.y + (point.y * bounds.height) / viewport.height,
              id: 1,
            },
          ],
        });
        if (index === 0 && holdMilliseconds > 0) {
          const { promise, resolve } = Promise.withResolvers<void>();
          setTimeout(resolve, holdMilliseconds);
          await promise;
        }
        // The first move must clear long-press jitter before yielding: a
        // stalled initial frame can otherwise exceed 500 ms and select text.
        // Later steps use real frames so Chrome can recognize a native pan.
        if (index > 0) {
          // CDP acknowledgement and animation frames do not guarantee the
          // last move reached the page under load. Ending the finger early
          // can discard that move and lose a row in exact wheel assertions.
          await testFrame.waitForFunction(
            ({ last, expected }) =>
              Math.abs(last.x - expected.x) < 1 &&
              Math.abs(last.y - expected.y) < 1,
            { last: delivered, expected: point },
            { timeout: 5000 },
          );
          await testFrame.evaluate(
            () =>
              new Promise<void>((resolve) =>
                requestAnimationFrame(() =>
                  requestAnimationFrame(() => resolve()),
                ),
              ),
          );
        }
      }
      await cdp.send("Input.dispatchTouchEvent", {
        type: finish === "cancel" ? "touchCancel" : "touchEnd",
        touchPoints: [],
      });
      await testFrame.evaluate(
        () =>
          new Promise<void>((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
          ),
      );
    } finally {
      try {
        await delivered.evaluate((point) => point.dispose());
      } finally {
        await delivered.dispose();
        try {
          await cdp.send("Emulation.setTouchEmulationEnabled", {
            enabled: false,
          });
        } finally {
          await cdp.detach();
        }
      }
    }
  },
);

export default mergeConfig(
  sharedConfig,
  defineConfig({
    optimizeDeps: {
      include: ["@xterm/xterm", "@xterm/addon-fit", "@xterm/addon-webgl"],
    },
    test: {
      maxWorkers: process.env.CI
        ? undefined
        : Math.min(4, availableParallelism()),
      include: ["test/browser/**/*.test.ts"],
      setupFiles: ["./test/browser/setup.ts"],
      browser: {
        enabled: true,
        headless: true,
        provider: playwright({ contextOptions: { locale: "zh-CN" } }),
        instances: [{ browser: "chromium" }],
        viewport: { width: 1024, height: 900 },
        commands: { nativeTouchGesture },
      },
      coverage: {
        ...sharedCoverage,
        reportsDirectory: ".vitest/coverage/browser",
        reporter: ["text-summary", "json", "json-summary", "html"],
      },
      // These tests drive real terminals, gestures and canvases; shared CI
      // runners need more than the 15 s default for the heaviest cases.
      testTimeout: 60_000,
      expect: {
        poll: {
          timeout: 15_000,
        },
      },
    },
  }),
);
