import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import CodeJsonPreview from "../../src/components/code/CodeJsonPreview.svelte";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

test("captured expansion and paging restore in a new preview", async () => {
  const text = JSON.stringify({
    items: Array.from({ length: 450 }, (_, index) => index),
    // `deep` is the first level that starts closed.
    nested: { deep: { value: "hidden" } },
  });
  const first = await render(CodeJsonPreview, { text, onsource: () => {} });
  await page.getByRole("button", { name: "再显示 200 项" }).click();
  await expect
    .element(page.getByText("200:", { exact: true }).element().parentElement!)
    .toHaveFocus();
  await expect.element(page.getByText("399:", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: /^deep/ }).click();
  await expect.element(page.getByText('"hidden"')).toBeVisible();
  const captured = first.component.capture();
  await first.unmount();

  await render(CodeJsonPreview, {
    text,
    onsource: () => {},
    restore: captured,
  });
  await expect.element(page.getByText("399:", { exact: true })).toBeVisible();
  await expect
    .element(page.getByText("400:", { exact: true }))
    .not.toBeInTheDocument();
  await expect.element(page.getByText('"hidden"')).toBeVisible();
});

test("paging focuses the next collection, including on the last batch", async () => {
  const values = Array.from({ length: 202 }, (_, index) =>
    index === 200 ? { flag: true } : index,
  );
  await render(CodeJsonPreview, {
    text: JSON.stringify({ values }),
    onsource: () => {},
  });
  const more = page.getByRole("button", { name: "再显示 200 项" });
  more.element().focus();
  await userEvent.keyboard("{Enter}");
  const next = page.getByRole("button", { name: /^200/ });
  await expect.element(next).toHaveFocus();
  await expect.element(more).not.toBeInTheDocument();
  await userEvent.keyboard("{Enter}");
  await expect.element(page.getByText("flag:", { exact: true })).toBeVisible();
});
