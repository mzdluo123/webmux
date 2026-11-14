import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TvGitListsHarness.svelte";
import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  tvMode.enabled = false;
});

for (const enabled of [true, false]) {
  test(`branch selection crosses sections with native keys (TV ${enabled})`, async () => {
    await page.viewport(1280, 720);
    await render(Harness);
    tvMode.enabled = enabled;
    const list = page.getByRole("listbox", { name: "分支列表", exact: true });
    const options = [
      ...list.element().querySelectorAll<HTMLElement>('[role="option"]'),
    ];
    expect(options.length).toBeGreaterThan(2);
    await userEvent.click(options[0]!);
    for (let index = 1; index < options.length; index++) {
      await userEvent.keyboard("{ArrowDown}");
      expect(document.activeElement).toBe(options[index]);
      expect(options[index]!.getAttribute("aria-selected")).toBe("true");
      expect(options[index - 1]!.getAttribute("aria-selected")).toBe("false");
      const branchName =
        options[index]!.querySelector("[title]")!.getAttribute("title")!;
      await expect
        .element(
          page.getByTestId("git-branch-detail-card").getByRole("heading"),
        )
        .toHaveTextContent(branchName);
    }
    await userEvent.keyboard("{ArrowUp}");
    expect(document.activeElement).toBe(options.at(-2));
    expect(options.at(-2)!.getAttribute("aria-selected")).toBe("true");
    await userEvent.keyboard("{Home}");
    expect(document.activeElement).toBe(options[0]);
    await userEvent.keyboard("{End}");
    expect(document.activeElement).toBe(options.at(-1));
    if (enabled) {
      for (
        let attempt = 0;
        attempt < 4 && list.element().contains(document.activeElement);
        attempt++
      ) {
        // A branch's adjacent action button is also a valid spatial stop.
        await userEvent.keyboard("{ArrowRight}");
      }
      expect(list.element().contains(document.activeElement)).toBe(false);
      expect(document.activeElement?.matches("button, a[href], input")).toBe(
        true,
      );
    }
  });
}

test("Git operations use D-pad selection and Back dismisses only the popup", async () => {
  await page.viewport(1280, 720);
  await render(Harness);
  const trigger = page.getByRole("button", {
    name: "展开 Git 操作记录与状态详情",
  });
  await trigger.click();
  const refresh = page.getByRole("button", { name: "刷新操作记录" });
  await expect.element(refresh).toHaveFocus();
  await userEvent.keyboard("{ArrowDown}");
  const operations = [
    ...document.querySelectorAll<HTMLButtonElement>("[data-operation-id]"),
  ];
  expect(document.activeElement).toBe(operations[0]);
  await userEvent.keyboard("{ArrowDown}{Enter}");
  expect(document.activeElement).toBe(operations[1]);
  expect(operations[1]!.getAttribute("aria-current")).toBe("true");
  const fixture = buildGitWorkbenchFixture("branches-occupied");
  await expect
    .element(
      page
        .getByTestId("git-operation-detail")
        .getByText(fixture.operations[1]!.summary!, { exact: true }),
    )
    .toBeVisible();
  await userEvent.keyboard("{Escape}");
  await expect
    .element(page.getByTestId("git-operations-popover"))
    .not.toBeInTheDocument();
  await expect.element(trigger).toHaveFocus();
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("0");
});
