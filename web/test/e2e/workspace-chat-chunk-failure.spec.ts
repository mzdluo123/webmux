import { test, expect } from "./fixtures";

test.use({ plugins: ["workspace", "chat", "files", "launch"] });

test("a failed chat entry leaves workspace home live sessions and terminal navigation usable", async ({
  page,
  request,
  workDir,
}) => {
  const created = await request.post("/api/sessions", {
    data: {
      cwd: workDir,
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
      name: "Live without chat history",
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const { session } = await created.json();
  const failures: string[] = [];
  page.on("pageerror", (error) => failures.push(error.message));
  let blocked = 0;
  await page.route(/\/assets\/chat-[^/]+\.js/, (route) => {
    blocked++;
    return route.abort();
  });
  await page.goto("/");
  const live = page
    .getByRole("main")
    .getByText("Live without chat history", { exact: true });
  await expect(live).toBeVisible();
  await expect(
    page.getByText(
      "聊天记录暂不可用；仍可使用实时会话。请重新加载页面以恢复聊天功能。",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "重新加载页面" }),
  ).toBeVisible();
  expect(blocked).toBeGreaterThan(0);
  expect(failures).toEqual([]);
  await live.click();
  await expect(page).toHaveURL(new RegExp(`/s/${session.id}$`));
});
