import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 320, height: 568 }, hasTouch: true });

test("touch selects and applies both transports at a narrow viewport", async ({
  page,
}) => {
  await page.goto(
    "/iframe.html?id=settings-official-relay--transport-selection&globals=locale:en;theme:dark",
  );
  const transport = page.getByRole("region", {
    name: "Host-to-node data transport",
  });
  const apply = transport.getByRole("button", { name: "Test and apply" });
  await expect(apply).toBeDisabled();
  for (const [protocol, name] of [
    ["QUIC/UDP", "Experimental QUIC/UDP"],
    ["WSS/TCP", "WSS/TCP (default)"],
  ] as const) {
    const testing = transport.getByText(
      `Testing ${protocol} before applying it…`,
    );
    await transport.getByRole("radio", { name, exact: true }).tap();
    await apply.tap();
    await expect(testing).toBeVisible();
    await expect(apply).toBeDisabled();
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true);
    await expect(testing).toBeHidden();
    await expect(
      transport.getByText("Transport test succeeded."),
    ).toBeVisible();
  }
});
