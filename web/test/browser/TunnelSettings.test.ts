import { afterEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import TunnelSettings from "../../src/components/TunnelSettings.svelte";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

test("defaults to the Named Tunnel steps", async () => {
  render(TunnelSettings);
  await expect.element(page.getByText("Named Tunnel（推荐）")).toBeVisible();
  await expect
    .element(page.getByText("Networking → Tunnels", { exact: false }))
    .toBeVisible();
  await expect
    .element(page.getByText("Routes → Add route", { exact: false }))
    .toBeVisible();
  await expect.element(page.getByText("http://127.0.0.1:7681")).toBeVisible();
  await expect.element(page.getByText("复制配置模板")).toBeVisible();
});

test("switches to the Quick Tunnel explanation", async () => {
  render(TunnelSettings);
  await page.getByRole("tab", { name: "Quick Tunnel" }).click();
  await expect
    .element(page.getByText("每次启动地址都会变化", { exact: false }))
    .toBeVisible();
  await expect
    .element(page.getByText("Networking → Tunnels", { exact: false }))
    .not.toBeVisible();
});

test("shows the configuration template and documentation link", async () => {
  render(TunnelSettings);
  const template = page.getByLabelText("Named Tunnel 配置模板").element();
  expect(template.textContent).toContain('token = "Cloudflare 隧道 Token"');
  expect(template.textContent).toContain('hostname = "term.example.com"');
  const link = page.getByRole("link", {
    name: /查看完整步骤与故障排查/,
  });
  await expect
    .element(link)
    .toHaveAttribute(
      "href",
      "https://github.com/mzdluo123/webmux/blob/main/docs/cloudflare-tunnel.md",
    );
  await expect.element(link).toHaveAttribute("target", "_blank");
  expect(page.getByRole("textbox").elements()).toHaveLength(0);
});

test("copies only the tunnel template", async () => {
  const writeText = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockResolvedValue();
  render(TunnelSettings);
  await page.getByRole("button", { name: "复制配置模板" }).click();
  await expect.element(page.getByText("已复制")).toBeVisible();
  expect(writeText).toHaveBeenCalledOnce();
  const copied = writeText.mock.calls[0][0];
  expect(copied).toContain("[tunnel]");
  expect(copied).toContain('hostname = "term.example.com"');
  expect(copied).not.toContain("eyJ");
});
