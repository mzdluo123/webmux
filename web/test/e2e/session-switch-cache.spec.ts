import { expect, expectConnected, openSessionSwitcher, test } from "./fixtures";

test.describe("session switch LRU cache", () => {
  test.skip(({ isMobile }) => isMobile, "desktop session switch cache");

  test("reuses mounted session pages across switches and absorbs background output", async ({
    page,
    request,
    workDir,
  }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    const create = async (name: string, cmd: string[]) => {
      const res = await request.post("/api/sessions", {
        data: { cmd, name, cwd: workDir },
      });
      expect(res.status(), await res.text()).toBe(201);
      return (await res.json()).session.id as string;
    };

    const idA = await create("CacheA", [
      "/bin/sh",
      "-c",
      "printf 'ALPHA_INITIAL\\n'; sleep 2; printf 'ALPHA_BACKGROUND\\n'; sleep 600",
    ]);
    const idB = await create("CacheB", [
      "/bin/sh",
      "-c",
      "printf 'BETA_READY\\n'; sleep 600",
    ]);

    let wsCountA = 0;
    page.on("websocket", (ws) => {
      if (ws.url().includes(`/ws/s/${idA}`)) wsCountA += 1;
    });

    await page.goto(`/s/${idA}`);
    await expectConnected(page);
    await expect(
      page.locator(".terminal-page:not([data-cache-hidden='true'])"),
    ).toContainText("ALPHA_INITIAL");
    expect(wsCountA).toBe(1);

    // Switch to session B via the session switcher.
    await openSessionSwitcher(page);
    await page.getByRole("option", { name: /CacheB/ }).click();
    await expect(page).toHaveURL(new RegExp(`/s/${idB}$`));
    await expect(
      page.locator(".terminal-page:not([data-cache-hidden='true'])"),
    ).toContainText("BETA_READY");

    // Session A remains mounted in the background cache and is hidden/inert.
    await expect(
      page.locator(".terminal-page[data-cache-hidden='true']"),
    ).toHaveCount(1);

    // Wait for session A to emit background output while B is active.
    await page.waitForTimeout(2500);
    await expect(page).toHaveTitle(/CacheB/);

    // Switch back to A: reuses existing WS connection and already has output.
    await openSessionSwitcher(page);
    await page.getByRole("option", { name: /CacheA/ }).click();
    await expect(page).toHaveURL(new RegExp(`/s/${idA}$`));
    await expect(
      page.locator(".terminal-page:not([data-cache-hidden='true'])"),
    ).toContainText("ALPHA_BACKGROUND");
    expect(wsCountA).toBe(1);
    await expect(page).toHaveTitle(/CacheA/);
  });
});
