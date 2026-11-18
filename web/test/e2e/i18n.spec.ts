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

    // Check the default new-session composer in English.
    await page.getByRole("button", { name: "New session" }).first().click();
    await expect(page).toHaveURL(/\/new$/);
    await expect(
      page.getByRole("textbox", { name: "Task description" }),
    ).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await page.goBack();

    // Enter real PTY session in English and verify terminal input/output works.
    let sawNativeDialog = false;
    page.on("dialog", (dialog) => {
      sawNativeDialog = true;
      void dialog.dismiss();
    });
    await page.getByRole("link", { name: "i18n-shell", exact: true }).click();
    await expect(page).toHaveURL(/\/s\/[a-z2-7]{8}$/);
    await runCommand(page, "printf 'I18N_PTY_OK\\n'");
    await expectTerminalLine(page, "I18N_PTY_OK");
    await expectNoHorizontalOverflow(page);

    // Emit an OSC 8 hyperlink and verify the confirmation dialog follows WebMux locale (en -> zh-CN) instead of native confirm().
    const linkUrl = "https://example.com/webmux-i18n-check";
    const sessionUrl = page.url();
    await runCommand(
      page,
      `printf '\\033[2J\\033[H\\033]8;;${linkUrl}\\033\\\\OSC8_TARGET\\033]8;;\\033\\\\\\n'`,
    );
    await expectTerminalLine(page, "OSC8_TARGET");
    const clickOscLink = async () => {
      const box = await page.locator(".xterm-screen").boundingBox();
      expect(box).not.toBeNull();
      const x = box!.x + 20;
      const y = box!.y + 8;
      await page.mouse.move(x, box!.y + 80);
      await page.mouse.move(x, y);
      await page.waitForTimeout(50);
      await page.mouse.down();
      await page.mouse.up();
    };
    await clickOscLink();
    const enLinkDialog = page.getByRole("alertdialog", {
      name: "Open external link?",
    });
    await expect(enLinkDialog).toBeVisible();
    await expect(enLinkDialog.getByText(linkUrl)).toBeVisible();
    await enLinkDialog
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(enLinkDialog).toBeHidden();
    expect(sawNativeDialog).toBe(false);

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

    // Verify explicit zh-CN preference also localizes the terminal OSC 8 link dialog on an en-US browser.
    await page
      .getByRole("radiogroup", { name: "Language", exact: true })
      .getByRole("radio", { name: "简体中文", exact: true })
      .click();
    await page.goto(sessionUrl);
    await runCommand(
      page,
      `printf '\\033[2J\\033[H\\033]8;;${linkUrl}\\033\\\\OSC8_TARGET\\033]8;;\\033\\\\\\n'`,
    );
    await expectTerminalLine(page, "OSC8_TARGET");
    await clickOscLink();
    const zhLinkDialog = page.getByRole("alertdialog", {
      name: "打开外部链接？",
    });
    await expect(zhLinkDialog).toBeVisible();
    await expect(zhLinkDialog.getByText(linkUrl)).toBeVisible();
    await zhLinkDialog
      .getByRole("button", { name: "取消", exact: true })
      .click();
    await expect(zhLinkDialog).toBeHidden();
    expect(sawNativeDialog).toBe(false);
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
