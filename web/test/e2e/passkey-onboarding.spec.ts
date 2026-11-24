import { test, expect, type Page } from "@playwright/test";
import { startE2EServer, type E2EServer } from "./server.mjs";

let server: E2EServer & { setupCode?: string; setupURL?: string };

async function attachVirtualAuthenticator(page: Page) {
  const client = await page.context().newCDPSession(page);
  await client.send("WebAuthn.enable");
  const { authenticatorId } = await client.send(
    "WebAuthn.addVirtualAuthenticator",
    {
      options: {
        protocol: "ctap2",
        transport: "internal",
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
        automaticPresenceSimulation: true,
      },
    },
  );
  return { client, authenticatorId };
}

test.describe("Passkey onboarding and login", () => {
  test.beforeAll(async () => {
    server = await startE2EServer({ onboarding: true });
  });

  test.afterAll(async () => {
    await server?.close();
  });

  test("completes onboarding via setup URL + real Passkey, signs out, and signs back in with Passkey", async ({
    page,
    browserName,
  }) => {
    test.skip(
      browserName !== "chromium",
      "CDP Virtual WebAuthn Authenticator requires Chromium",
    );
    expect(server.setupCode).toBeTruthy();
    expect(server.setupURL).toContain("http://localhost:");
    expect(server.setupURL).toContain("/onboarding#setup=");

    await attachVirtualAuthenticator(page);

    // 1. Open the printed onboarding setup link; #setup= is stripped automatically.
    await page.goto(server.setupURL!);
    await expect(
      page.getByRole("button", { name: /创建 Passkey|Create Passkey/ }),
    ).toBeVisible();
    expect(page.url()).not.toContain("#setup=");

    // 2. Register first Passkey via virtual authenticator.
    await page
      .getByRole("button", { name: /创建 Passkey|Create Passkey/ })
      .click();
    await expect(
      page.getByText(
        /请妥善保存一次性恢复码|Save your one-time recovery codes/,
      ),
    ).toBeVisible();

    // Capture a recovery code for later verification.
    const recoveryText = await page
      .locator("span.select-all")
      .first()
      .textContent();
    expect(recoveryText?.trim()).toMatch(/^[A-Z2-9]{4}-[A-Z2-9]{4}$/);

    // 3. Walk through Step 3 (Workspace) and Step 4 (Remote Access) -> Enter WebMux.
    await page.getByRole("button", { name: /下一步|Continue/ }).click();
    await expect(
      page.getByText(/确认工作环境|Confirm your workspace/),
    ).toBeVisible();
    await page.getByRole("button", { name: /下一步|Continue/ }).click();
    await expect(
      page.getByText(/手机与远程访问|Connect from your phone/),
    ).toBeVisible();
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page
      .getByRole("button", { name: /进入 WebMux|Enter WebMux/ })
      .click();

    // Workspace is now accessible without any hardcoded config.toml token, and
    // the onboarding enter transition finishes cleanly without leaving state behind.
    await expect(page.getByRole("textbox", { name: "任务描述" })).toBeVisible();
    await expect(page.locator("html")).not.toHaveAttribute(
      "data-onboarding-transition",
    );

    // 4. Open Settings -> Security & Login, verify passkey & session appear, then sign out.
    await page.goto(`${server.baseURL}/settings`);
    await expect(
      page.getByRole("heading", { name: /Passkey 凭据|Passkeys/ }),
    ).toBeVisible();
    await expect(page.getByText("localhost").first()).toBeVisible();

    // Sign out current device -> lands on Passkey Login screen.
    await page
      .getByRole("button", { name: /退出当前设备|Sign out this device/ })
      .click();
    await expect(
      page.getByRole("button", {
        name: /使用 Passkey 登录|Sign in with Passkey/,
      }),
    ).toBeVisible();

    // 5. Sign back in using the registered Passkey (returns to /settings where we signed out).
    await page
      .getByRole("button", {
        name: /使用 Passkey 登录|Sign in with Passkey/,
      })
      .click();
    await expect(
      page.getByRole("heading", { name: /Passkey 凭据|Passkeys/ }),
    ).toBeVisible();

    // Verify no horizontal overflow on onboarding/settings view.
    const hasOverflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
    expect(hasOverflow).toBe(false);

    // 6. Test Step-Up + Pairing code generation in Settings.
    await page
      .getByRole("button", { name: /生成配对码|Generate pairing code/ })
      .click();
    await page
      .getByRole("button", { name: /生成配对码|Create pairing code/ })
      .last()
      .click();
    await expect(
      page.getByRole("button", { name: /复制配对码|Copy code/ }),
    ).toBeVisible();
  });
});
