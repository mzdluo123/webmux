import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { render } from "vitest-browser-svelte/pure";
import Terminal from "../../src/pages/Terminal.svelte";
import type { SessionInfo } from "../../src/contracts/schemas";
import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
import { initializeLocale, locale } from "../../src/lib/locale";
import {
  installChatFixture,
  respond,
  socketAt,
  type FakeSocket,
} from "./chat-fixture";
import { seedForwards } from "./forward-fixture";

installChatFixture();

const id = "linkterm";
const terminalPath = `/ws/s/${id}`;

const sessionInfo: SessionInfo = {
  id,
  generation: "0123456789abcdef",
  title: "Shell · link check",
  cwd: "/work",
  cmd: ["/bin/sh"],
  status: "idle",
  exitCode: null,
  listeningPorts: [],
  state: "active",
  views: ["terminal"],
  view: "terminal",
  transition: null,
  agent: null,
  preview: "",
  createdAt: "2026-11-16T00:00:00Z",
  lastActiveAt: "2026-11-16T00:00:00Z",
  cols: 80,
  rows: 24,
  clients: 1,
  workspace: null,
  attention: null,
  reportedCwd: null,
};

function receivePty(socket: FakeSocket, kind: number, text: string) {
  const payload = new TextEncoder().encode(text);
  const frame = new Uint8Array(payload.length + 1);
  frame[0] = kind;
  frame.set(payload, 1);
  socket.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
}

beforeEach(() => {
  seedForwards([]);
  initializeLocale();
  locale.setPreference("zh-CN");
});

afterEach(async () => {
  vi.restoreAllMocks();
  locale.setPreference("zh-CN");
  await page.viewport(1280, 900);
});

async function mountShellTerminal(): Promise<FakeSocket> {
  respond(/^GET \/api\/sessions\/linkterm$/, () => Response.json(sessionInfo));
  const drafts = createTerminalDrafts();
  render(Terminal, {
    id,
    drafts,
    onUploadNotice: () => {},
  });
  const terminal = await socketAt(terminalPath);
  const clientId = new URL(terminal.url).searchParams.get("client");
  terminal.binary(2, sessionInfo);
  terminal.binary(6, {
    controller: {
      clientId,
      label: "fixture",
      since: "2026-11-16T00:00:00Z",
    },
    cols: 80,
    rows: 24,
    colorSchemeReports: false,
  });
  receivePty(terminal, 1, "$ ");
  await expect
    .poll(
      () =>
        document.querySelector<HTMLElement>(".xterm-screen")?.clientWidth ?? 0,
    )
    .toBeGreaterThan(0);
  return terminal;
}

async function activateFirstRowLink() {
  const screen = document.querySelector<HTMLElement>(".xterm-screen")!;
  const rect = screen.getBoundingClientRect();
  // `$ ` occupies columns 0..1; +28px, +8px lands inside columns 2..5 of row 0.
  const clientX = rect.left + 28;
  const clientY = rect.top + 8;
  // Move onto a different row first so xterm's Linkifier refreshes row 0's OSC 8 cache.
  screen.dispatchEvent(
    new MouseEvent("mousemove", {
      bubbles: true,
      clientX,
      clientY: rect.top + 80,
    }),
  );
  screen.dispatchEvent(
    new MouseEvent("mousemove", { bubbles: true, clientX, clientY }),
  );
  // Allow xterm's OscLinkProvider reply to settle.
  await new Promise((resolve) => setTimeout(resolve, 40));
  screen.dispatchEvent(
    new MouseEvent("mousedown", { bubbles: true, button: 0, clientX, clientY }),
  );
  screen.dispatchEvent(
    new MouseEvent("mouseup", { bubbles: true, button: 0, clientX, clientY }),
  );
}

test("OSC 8 link uses localized ConfirmDialog, updates live across locales, and does not block PTY output", async () => {
  await page.viewport(1280, 900);
  const confirmSpy = vi.spyOn(window, "confirm");
  const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

  const url = "https://github.com/mzdluo123/webmux/pull/657";
  const terminal = await mountShellTerminal();
  receivePty(terminal, 0, `\x1b]8;;${url}\x1b\\OPEN_PR\x1b]8;;\x1b\\\r\n`);
  await new Promise((resolve) => setTimeout(resolve, 40));

  await activateFirstRowLink();

  expect(confirmSpy).not.toHaveBeenCalled();
  const zhDialog = page.getByRole("alertdialog", { name: "打开外部链接？" });
  await expect.element(zhDialog).toBeVisible();
  await expect.element(zhDialog.getByText("是否前往以下链接？")).toBeVisible();
  await expect.element(zhDialog.getByText(url)).toBeVisible();
  await expect
    .element(
      zhDialog.getByText(
        "注意：此链接来自终端输出，可能存在安全风险，请确认目标地址可信。",
      ),
    )
    .toBeVisible();

  // Live PTY output continues streaming while the modal is open without blocking.
  receivePty(terminal, 0, "STREAM_WHILE_MODAL_OPEN\r\n");

  // Switching WebMux locale updates the open confirmation dialog immediately.
  locale.setPreference("en");
  const enDialog = page.getByRole("alertdialog", {
    name: "Open external link?",
  });
  await expect.element(enDialog).toBeVisible();
  await expect
    .element(enDialog.getByText("Do you want to navigate to this link?"))
    .toBeVisible();
  await expect.element(enDialog.getByText(url)).toBeVisible();
  await expect
    .element(
      enDialog.getByText(
        "Warning: This link comes from terminal output and could potentially be dangerous.",
      ),
    )
    .toBeVisible();

  // Cancel does not open the link.
  await enDialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect.element(enDialog).not.toBeInTheDocument();
  expect(openSpy).not.toHaveBeenCalled();

  // Re-activate in English and confirm opening the link.
  await activateFirstRowLink();
  await expect.element(enDialog).toBeVisible();
  await enDialog
    .getByRole("button", { name: "Open link", exact: true })
    .click();
  await expect.element(enDialog).not.toBeInTheDocument();
  expect(openSpy).toHaveBeenCalledTimes(1);
  expect(openSpy).toHaveBeenCalledWith(url, "_blank", "noopener,noreferrer");
  expect(confirmSpy).not.toHaveBeenCalled();
});

test("long OSC 8 URL wraps cleanly inside 320px viewport and non-http schemes are ignored", async () => {
  await page.viewport(320, 568);
  const confirmSpy = vi.spyOn(window, "confirm");
  const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

  const terminal = await mountShellTerminal();
  receivePty(
    terminal,
    0,
    "\x1b]8;;javascript:alert(1)\x1b\\BADJS\x1b]8;;\x1b\\\r\n",
  );
  await new Promise((resolve) => setTimeout(resolve, 40));
  await activateFirstRowLink();
  expect(confirmSpy).not.toHaveBeenCalled();
  expect(openSpy).not.toHaveBeenCalled();
  expect(document.querySelector('[role="alertdialog"]')).toBeNull();

  const longUrl =
    "https://example.com/very/long/path/segment/that/must/wrap/cleanly/inside/iphone-se-320px-viewport?token=alpha-beta-gamma-delta-epsilon-0123456789";
  receivePty(
    terminal,
    0,
    `\x1b[2J\x1b[H$ \x1b]8;;${longUrl}\x1b\\LONG\x1b]8;;\x1b\\`,
  );
  await new Promise((resolve) => setTimeout(resolve, 40));
  await activateFirstRowLink();

  const dialog = page.getByRole("alertdialog", { name: "打开外部链接？" });
  await expect.element(dialog).toBeVisible();
  const rect = dialog.element().getBoundingClientRect();
  expect(rect.left).toBeGreaterThanOrEqual(0);
  expect(rect.right).toBeLessThanOrEqual(320);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});
