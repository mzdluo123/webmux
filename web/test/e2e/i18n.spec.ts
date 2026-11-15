import { expect, runCommand, expectTerminalLine, test } from "./fixtures";
import { startE2EServer } from "./server.mjs";

async function expectNoHorizontalOverflow(
  page: import("@playwright/test").Page,
) {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - window.innerWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
}

test.describe("English system locale and live switching", () => {
  test.use({ locale: "en-US" });

  test("boots in English by default, switches live in Settings, and persists across reload without server sync", async ({
    page,
    request,
    workDir,
  }) => {
    const created = await request.post("/api/sessions", {
      data: {
        cwd: workDir,
        cmd: ["/bin/sh"],
        name: "i18n-shell",
      },
    });
    expect(created.status()).toBe(201);

    await page.goto("/");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(
      page.getByRole("button", { name: "New session" }).first(),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);

    // Open New Session dialog in English and verify no horizontal overflow.
    await page.getByRole("button", { name: "New session" }).first().click();
    await expect(
      page.getByRole("dialog", { name: "New session" }),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await page.keyboard.press("Escape");

    // Enter real PTY session in English and verify terminal input/output works.
    await page.getByRole("link", { name: "i18n-shell", exact: true }).click();
    await expect(page).toHaveURL(/\/s\/[a-z2-7]{8}$/);
    await runCommand(page, "printf 'I18N_PTY_OK\\n'");
    await expectTerminalLine(page, "I18N_PTY_OK");
    await expectNoHorizontalOverflow(page);

    // Open Settings -> Appearance & reading in English, verify Follow system is selected.
    await page.goto("/settings");
    const localeGroup = page.getByRole("radiogroup", {
      name: "Language",
      exact: true,
    });
    await expect(localeGroup).toBeVisible();
    await expect(
      localeGroup.getByRole("radio", { name: "Follow system", exact: true }),
    ).toHaveAttribute("aria-checked", "true");
    await expectNoHorizontalOverflow(page);

    // Switch live to Simplified Chinese without reload.
    await localeGroup
      .getByRole("radio", { name: "简体中文", exact: true })
      .click();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    await expect(
      page.locator("#appearance").getByText("外观与阅读", { exact: true }),
    ).toBeVisible();

    // Reload: explicit zh-CN preference persists on an en-US browser, and server settings stay untouched.
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    await expect(
      page.locator("#appearance").getByText("外观与阅读", { exact: true }),
    ).toBeVisible();
    const serverSettings = await (await request.get("/api/settings")).json();
    expect(serverSettings.values).not.toHaveProperty("locale");

    // Switch back to Follow system -> immediately returns to English.
    await page
      .getByRole("radiogroup", { name: "语言", exact: true })
      .getByRole("radio", { name: "跟随系统", exact: true })
      .click();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(
      page
        .locator("#appearance")
        .getByText("Appearance & reading", { exact: true }),
    ).toBeVisible();
  });

  test("narrow 320px mobile viewport renders English views without horizontal overflow", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    await page.goto("/");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(
      page.getByRole("button", { name: "New session" }).first(),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await page.goto("/settings");
    await expect(
      page
        .locator("#appearance")
        .getByText("Appearance & reading", { exact: true }),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);
  });
});

test("token-protected ConnectionGate switches language live before login", async ({
  browser,
}) => {
  const token = "i18n-gate-shared-secret-token-0123456789abcdef";
  const gatedServer = await startE2EServer({ token });
  const context = await browser.newContext({
    baseURL: gatedServer.baseURL,
    locale: "zh-CN",
  });
  try {
    const page = await context.newPage();
    await page.goto("/");
    await expect(page.getByText("你的终端，始终在线。")).toBeVisible();

    await page.getByLabel("共享 Token", { exact: true }).fill(token);
    await page.getByRole("radio", { name: "English", exact: true }).click();

    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByText("Your terminal, always online.")).toBeVisible();
    await expect(page.getByLabel("Shared Token", { exact: true })).toHaveValue(
      token,
    );
    await expectNoHorizontalOverflow(page);

    await page
      .getByRole("button", { name: "Connect to WebMux →", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "New session" }).first(),
    ).toBeVisible();
  } finally {
    await context.close();
    await gatedServer.close();
  }
});
