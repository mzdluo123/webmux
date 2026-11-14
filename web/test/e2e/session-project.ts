import { expect, type Locator, type Page } from "@playwright/test";

const projectSearch = (page: Page) =>
  page.getByRole("combobox", { name: "搜索项目", exact: true });
export const sessionLocation = (page: Page) =>
  page.getByRole("button", { name: /^(工作)?位置$/ });

async function localSource(page: Page) {
  const add = page.getByRole("button", { name: "添加项目…", exact: true });
  const local = page.getByRole("option", { name: /^本地文件夹/ });
  await expect(add.or(local).first()).toBeVisible();
  if (await add.isVisible()) await add.click();
  await local.click();
}

/** Selects a service-side folder through the real project registration flow. */
export async function chooseSessionDirectory(page: Page, cwd?: string) {
  if (!(await projectSearch(page).isVisible())) {
    await page.getByRole("button", { name: "更换项目", exact: true }).click();
  }
  await expect(projectSearch(page)).toBeVisible();
  await localSource(page);
  if (cwd !== undefined)
    await page.getByLabel("已有文件夹", { exact: true }).fill(cwd);
  const submit = page.getByRole("button", {
    name: "打开文件夹并继续",
    exact: true,
  });
  await submit.click();
  await expect(submit).toBeHidden();
}

export async function chooseSessionProject(page: Page, name: string) {
  if (!(await projectSearch(page).isVisible()))
    await page.getByRole("button", { name: "更换项目", exact: true }).click();
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  await page
    .getByRole("option", { name: new RegExp(`^${escaped}(?:\\s|$)`) })
    .click();
  await expect(projectSearch(page)).toBeHidden();
}

/** Global creation now starts at projects; explicit contexts already have setup. */
export async function ensureSessionSetup(page: Page) {
  const ready: Locator = projectSearch(page)
    .or(page.getByRole("button", { name: "运行", exact: true }))
    .or(page.getByRole("textbox", { name: "自定义启动命令", exact: true }))
    .or(sessionLocation(page));
  await expect(ready.first()).toBeVisible();
  if (await projectSearch(page).isVisible()) await chooseSessionDirectory(page);
}
