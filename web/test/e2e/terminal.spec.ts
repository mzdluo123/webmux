import { setTimeout as delay } from "node:timers/promises";
import type { Page } from "@playwright/test";
import {
  test,
  expect,
  createShell,
  runCommand,
  expectTerminalLine,
} from "./fixtures";

type Point = { x: number; y: number };

async function textPoint(
  page: Page,
  line: string,
  text: string,
): Promise<Point> {
  return page.locator(".xterm-rows > div").evaluateAll(
    (rows, target) => {
      const row = rows.find(
        (element) =>
          element.textContent?.replaceAll("\u00a0", " ").trimEnd() ===
          target.line,
      );
      if (!row) throw new Error(`Missing rendered line: ${target.line}`);
      const walker = document.createTreeWalker(row, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        const offset = node.textContent?.indexOf(target.text) ?? -1;
        if (offset < 0) continue;
        const range = document.createRange();
        range.setStart(node, offset);
        range.setEnd(node, offset + target.text.length);
        const bounds = range.getBoundingClientRect();
        return {
          x: bounds.x + bounds.width * 0.75,
          y: bounds.y + bounds.height / 2,
        };
      }
      throw new Error(`Missing rendered text: ${target.text}`);
    },
    { line, text },
  );
}

async function longPress(page: Page, start: Point, end?: Point) {
  const cdp = await page.context().newCDPSession(page);
  try {
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ ...start, id: 1 }],
    });
    await delay(650);
    if (end)
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchMove",
        touchPoints: [{ ...end, id: 1 }],
      });
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchEnd",
      touchPoints: [],
    });
  } finally {
    await cdp.detach();
  }
}

test("dark theme corrects explicit black ANSI input and survives reload", async ({
  page,
  workDir,
}) => {
  await page.addInitScript(() => localStorage.setItem("webmux.theme", "dark"));
  await createShell(page, "Dark contrast", workDir);
  await runCommand(
    page,
    "printf '\\033[38;2;0;0;0m%s %s\\033[0m\\n' 'dark-input' '中文'",
  );
  await expectTerminalLine(page, "dark-input 中文");
  const contrast = () =>
    page.locator(".xterm-rows span").evaluateAll((spans) => {
      const span = spans.find((element) =>
        element.textContent?.startsWith("dark-input"),
      );
      if (!span) return 0;
      const luminance = (color: string) => {
        const rgb = color
          .match(/\d+/g)!
          .slice(0, 3)
          .map((value) => {
            const channel = Number(value) / 255;
            return channel <= 0.04045
              ? channel / 12.92
              : ((channel + 0.055) / 1.055) ** 2.4;
          });
        return rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722;
      };
      const foreground = luminance(getComputedStyle(span).color);
      const background = luminance(
        getComputedStyle(document.querySelector(".terminal-area")!)
          .backgroundColor,
      );
      return (
        (Math.max(foreground, background) + 0.05) /
        (Math.min(foreground, background) + 0.05)
      );
    });
  await expect.poll(contrast).toBeGreaterThanOrEqual(4.49);
  await page.reload();
  await expectTerminalLine(page, "dark-input 中文");
  await expect.poll(contrast).toBeGreaterThanOrEqual(4.49);
});

test("mobile long press selects wide characters and drag copies multiple lines without typing", async ({
  page,
  context,
  request,
  workDir,
}, testInfo) => {
  test.skip(
    !testInfo.project.use.isMobile,
    "Native touch selection requires the mobile project",
  );
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const id = await createShell(page, "Touch selection", workDir);
  await runCommand(
    page,
    "printf '%s %s %s\\n%s %s\\n' 'prefix' '中文' 'tail' 'second' 'line'",
  );
  await expectTerminalLine(page, "prefix 中文 tail");
  await expectTerminalLine(page, "second line");
  await page
    .locator(".xterm-helper-textarea")
    .evaluate((element) => (element as HTMLTextAreaElement).blur());
  const buffer = await (await request.get(`/api/sessions/${id}/buffer`)).text();
  const start = await textPoint(page, "prefix 中文 tail", "中");
  await longPress(page, start);
  await expect(
    page.getByRole("button", { name: "复制选中", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".xterm-helper-textarea")).not.toBeFocused();
  await page.getByRole("button", { name: "复制选中", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("中文");
  const end = await textPoint(page, "second line", "d");
  await longPress(page, start, end);
  await page.getByRole("button", { name: "复制选中", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("中文 tail\nsecond");
  await testInfo.attach("mobile-selection.png", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
  await expect(page.locator(".xterm-helper-textarea")).not.toBeFocused();
  expect(await (await request.get(`/api/sessions/${id}/buffer`)).text()).toBe(
    buffer,
  );
  await page.getByRole("button", { name: "取消选择", exact: true }).click();
  await expect(
    page.getByRole("toolbar", { name: "终端文字选择" }),
  ).toBeHidden();
});

test("mobile denied clipboard exposes only selected text for native copying", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(
    !testInfo.project.use.isMobile,
    "Native touch selection requires the mobile project",
  );
  await page.addInitScript(() => {
    Object.defineProperty(navigator.clipboard, "writeText", {
      value: () =>
        Promise.reject(
          new DOMException("Permission denied", "NotAllowedError"),
        ),
    });
  });
  await createShell(page, "Clipboard fallback", workDir);
  await runCommand(page, "printf '%s %s\\n' 'prefix' 'selected_word'");
  await expectTerminalLine(page, "prefix selected_word");
  await longPress(
    page,
    await textPoint(page, "prefix selected_word", "selected_word"),
  );
  await page.getByRole("button", { name: "复制选中", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "复制终端文字" }),
  ).toBeVisible();
  await expect(page.getByRole("textbox", { name: "复制内容" })).toHaveValue(
    "selected_word",
  );
});

function osc52Command(text: string) {
  const payload = Buffer.from(text, "utf8").toString("base64");
  return `printf '\\033]52;c;${payload}\\a'`;
}

test("application OSC 52 copy is written to the browser clipboard", async ({
  page,
  context,
  workDir,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await createShell(page, "OSC 52 copy", workDir);
  await runCommand(page, osc52Command("osc52-中文"));
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("osc52-中文");
  await expect(page.locator(".xterm-rows")).toContainText("printf");
  await expect(page.locator(".xterm-rows")).not.toContainText("osc52-中文");
});

test("denied OSC 52 copy exposes the text for manual copying", async ({
  page,
  workDir,
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator.clipboard, "writeText", {
      configurable: true,
      value: () =>
        Promise.reject(
          new DOMException("Permission denied", "NotAllowedError"),
        ),
    });
  });
  await createShell(page, "OSC 52 fallback", workDir);
  await runCommand(page, osc52Command("osc52_fallback"));
  await expect(
    page.getByRole("dialog", { name: "复制终端文字" }),
  ).toBeVisible();
  await expect(page.getByRole("textbox", { name: "复制内容" })).toHaveValue(
    "osc52_fallback",
  );
});
