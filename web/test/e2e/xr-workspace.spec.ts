import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import type { XrPanelLayout } from "../../src/domains/terminal/xr-layout";
import { runCommand } from "./fixtures";
import type { E2EServer } from "./server.mjs";
import { expect, holdChatReattach, test, type XrBrowser } from "./xr-fixtures";

// The ordinary E2E project disables GL for xterm DOM assertions. This isolated
// spec needs real software WebGL for Three's production immersive frame loop.
test.use({
  launchOptions: {
    args: [
      "--use-gl=angle",
      "--use-angle=swiftshader",
      "--enable-unsafe-swiftshader",
    ],
  },
});

async function createChat(
  request: APIRequestContext,
  cwd: string,
  name: string,
  permission = false,
) {
  const config = await request.post("/api/launch-configs", {
    data: {
      name,
      cmd: [permission ? "claude" : "omp"],
      chat: true,
      env: {
        FAKEAGENT_TURN_DELAY: "30ms",
        ...(permission ? { FAKEAGENT_PERMISSION: "1" } : {}),
      },
    },
  });
  expect(config.status(), await config.text()).toBe(201);
  const response = await request.post("/api/sessions", {
    data: { configId: (await config.json()).id, cwd, name, view: "chat" },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()).session.id as string;
}

async function turn(page: Page, text: string) {
  await page.getByRole("textbox", { name: "消息", exact: true }).fill(text);
  await page.getByRole("button", { name: /^(发送|加入排队)$/ }).click();
  await expect(
    page.getByRole("list", { name: "对话", exact: true }),
  ).toContainText(`echo: ${text}`);
  await expect(
    page.getByRole("status").filter({ hasText: "正在工作…" }),
  ).toHaveCount(0);
}

async function received(
  request: APIRequestContext,
  server: E2EServer,
  id: string,
) {
  const info = await (await request.get(`/api/sessions/${id}`)).json();
  const stored = JSON.parse(
    await readFile(
      join(
        server.env.FAKEAGENT_STORE!,
        "sessions",
        `${info.agent.sessionId}.json`,
      ),
      "utf8",
    ),
  );
  return (stored.messages as { role: string; text: string }[])
    .filter((message) => message.role === "user")
    .map((message) => message.text);
}

async function visibleChat(
  xr: XrBrowser,
  sessionId: string,
): Promise<XrPanelLayout[]> {
  return (await xr.layouts()).filter(
    (layout) =>
      layout.sessionId === sessionId &&
      layout.kind === "chat" &&
      layout.visible,
  );
}

async function chatLayout(xr: XrBrowser, sessionId: string) {
  await expect
    .poll(async () => (await visibleChat(xr, sessionId)).length)
    .toBeGreaterThan(0);
  return (await visibleChat(xr, sessionId))[0]!;
}

async function viewSurface(xr: XrBrowser, layout: XrPanelLayout) {
  // View identity is user-visible in the painted title, not a Svelte internals hook.
  return xr.findSurface(layout.id.slice(-8));
}

async function expectPainted(
  xr: XrBrowser,
  layout: XrPanelLayout,
  text: string,
) {
  const surface = await viewSurface(xr, layout);
  await expect.poll(() => xr.text(surface.id)).toContain(text);
  return surface.id;
}

async function active2d(page: Page) {
  return page
    .locator('.split-terminal[data-active="true"]')
    .getAttribute("data-session-id");
}

async function expectShellOutput(
  request: APIRequestContext,
  id: string,
  text: string,
) {
  await expect
    .poll(async () => (await request.get(`/api/sessions/${id}/buffer`)).text())
    .toContain(text);
}

test("IWER A1 history, A2 tail and B share owners while VR-only physical/virtual input stays unique", async ({
  page,
  request,
  browser,
  server,
  workDir,
  xr,
}, info) => {
  test.skip(
    !!info.project.use.isMobile,
    "Immersive software-GL desktop runtime; real headset remains separate acceptance",
  );
  test.setTimeout(120_000);
  const a = await createChat(request, workDir, "XR A");
  const b = await createChat(request, workDir, "XR B");
  const shellResponse = await request.post("/api/sessions", {
    data: {
      cwd: workDir,
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
      name: "XR grid controller",
    },
  });
  expect(shellResponse.status()).toBe(201);
  const shell = (await shellResponse.json()).session.id as string;
  const other = await browser.newContext({
    baseURL: server.baseURL,
    viewport: { width: 1440, height: 900 },
  });
  const reconnect = await holdChatReattach(page);
  try {
    const remoteA = await other.newPage();
    const remoteB = await other.newPage();
    const controller = await other.newPage();
    await remoteA.goto(`/s/${a}`);
    await turn(
      remoteA,
      Array.from(
        { length: 30 },
        (_, index) => `A_HISTORY_${String(index).padStart(2, "0")}`,
      ).join("\n"),
    );
    await remoteB.goto(`/s/${b}`);
    await turn(remoteB, "B_INITIAL");
    await controller.goto(`/s/${shell}`);
    await expect(controller.locator(".xterm")).toBeVisible();
    await runCommand(controller, "printf 'XR_%s_CONTINUITY\\n' BEFORE");
    await expectShellOutput(request, shell, "XR_BEFORE_CONTINUITY");
    const grid = async () => {
      const session = await (
        await request.get(`/api/sessions/${shell}`)
      ).json();
      return [session.cols, session.rows];
    };
    const initialGrid = await grid();
    await page.goto(`/s/${shell}`);
    await expect(page.locator(".xterm")).toBeVisible();
    await expect(page.locator(".restore-overlay")).toBeHidden();
    const initialRoute = page.url();
    const initialActive = await active2d(page);
    await xr.enter();
    await xr.menu("XR A");
    const a1 = await chatLayout(xr, a);
    await expectPainted(xr, a1, "A_HISTORY_29");
    await xr.menu("XR B");
    const b1 = await chatLayout(xr, b);
    await expectPainted(xr, b1, "B_INITIAL");
    // Only the shell belongs to 2D: A's editor is an XR-only stable owner.
    await expect(page.locator(".split-terminal")).toHaveCount(1);
    expect(await active2d(page)).toBe(initialActive);
    expect(page.url()).toBe(initialRoute);
    expect(await grid()).toEqual(initialGrid);

    // Use a standard thumbstick and explicit history control, not panel methods.
    for (let index = 0; index < 10; index++) await xr.panel(a1, 80, 87);
    const a1Surface = await viewSurface(xr, a1);
    const history = (await xr.surface(a1Surface.id))!.text.filter(
      (line) => line.y >= 120 && line.y < 640,
    );
    expect(history.map((line) => line.text).join("\n")).toContain("A_HISTORY_");
    expect(history.map((line) => line.text).join("\n")).not.toContain(
      "A_HISTORY_29",
    );
    await xr.panel(a1, 700, 736); // Split view: the new view follows current tail.
    await expect.poll(async () => (await visibleChat(xr, a)).length).toBe(2);
    let a2 = (await visibleChat(xr, a)).find((view) => view.id !== a1.id)!;
    await expectPainted(xr, a2, "A_HISTORY_29");
    await turn(remoteA, "A_LIVE_ONE");
    await turn(remoteB, "B_LIVE_ONE");
    await expectPainted(xr, a2, "echo: A_LIVE_ONE");
    await expectPainted(xr, b1, "echo: B_LIVE_ONE");
    expect(
      (await xr.surface(a1Surface.id))!.text.filter(
        (line) => line.y >= 120 && line.y < 640,
      ),
    ).toEqual(history);
    // Move and resize only A2 through held tracked-pointer selections.
    const beforeTransform = { position: [...a2.position], size: [...a2.size] };
    // Start in front of every panel so an occluding original cannot be bypassed.
    await xr.point(
      [
        a2.position[0],
        a2.position[1] + a2.size[1] * (0.5 - 33 / 768),
        a2.position[2],
      ],
      -0.4,
    );
    await xr.trigger(1);
    await xr.point(
      [
        a2.position[0] + 0.18,
        a2.position[1] + a2.size[1] * (0.5 - 33 / 768) - 0.1,
        a2.position[2],
      ],
      -0.4,
    );
    await xr.trigger(0);
    a2 = (await visibleChat(xr, a)).find((view) => view.id === a2.id)!;
    expect(a2.position).not.toEqual(beforeTransform.position);
    const corner: [number, number, number] = [
      a2.position[0] + (950 / 1024 - 0.5) * a2.size[0],
      a2.position[1] + (0.5 - 736 / 768) * a2.size[1],
      a2.position[2],
    ];
    await xr.point(corner);
    await xr.trigger(1);
    await xr.point([corner[0] + 0.1, corner[1] - 0.05, corner[2]]);
    await xr.trigger(0);
    a2 = (await visibleChat(xr, a)).find((view) => view.id === a2.id)!;
    expect(a2.size).not.toEqual(beforeTransform.size);
    expect((await xr.layouts()).find((view) => view.id === a1.id)).toEqual(a1);
    expect(await grid()).toEqual(initialGrid);

    await xr.panel(a1, 460, 87); // Explicitly select A1 for both keyboard types.
    await page.keyboard.type("shared");
    // Merely scrolling B must not steal A's physical keyboard target.
    await xr.scroll(b1, -1);
    await page.keyboard.type("draft");
    await xr.panel(a2, 460, 87);
    await xr.menu("VR 键盘");
    await xr.key("v");
    await xr.key("Shift");
    await xr.key("↵");
    await expect(
      page
        .locator(`[data-session-id="${a}"]`)
        .getByRole("textbox", { name: "消息", exact: true }),
    ).toHaveValue("shareddraftv\n");
    expect(await received(request, server, a)).not.toContain("shareddraftv");
    await xr.key("⌫");
    await xr.key("↵");
    await expectPainted(xr, a2, "echo: shareddraftv");
    expect(
      (await received(request, server, a)).filter(
        (text) => text === "shareddraftv",
      ),
    ).toHaveLength(1);
    expect(await received(request, server, b)).not.toContain("shareddraftv");
    await xr.panel(b1, 460, 87);
    await page.keyboard.type("onlyb");
    await page.keyboard.press("Enter");
    await expectPainted(xr, b1, "echo: onlyb");
    expect(
      (await received(request, server, b)).filter((text) => text === "onlyb"),
    ).toHaveLength(1);
    expect(await received(request, server, a)).not.toContain("onlyb");

    // A1's Latest only changes A1; A2 remains a separate live view.
    await xr.panel(a1, 300, 87);
    await expectPainted(xr, a1, "echo: shareddraftv");
    await xr.panel(a2, 460, 87);
    await page.keyboard.type("retained");
    await xr.panel(a2, 900, 33);
    await page.keyboard.type("mustnotroute");
    await page.keyboard.press("Enter");
    expect(await received(request, server, a)).not.toContain(
      "retainedmustnotroute",
    );
    expect(await received(request, server, b)).not.toContain("mustnotroute");
    await xr.panel(a1, 900, 33);
    expect(await visibleChat(xr, a)).toHaveLength(0);
    // Reopen the last hidden view, preserving the canonical shared draft.
    await xr.menu("XR A");
    await xr.menu(`隐藏 chat · chat · 视图 ${a1.id.slice(-8)}`);
    const reopened = await chatLayout(xr, a);
    await expectPainted(xr, reopened, "retained");
    await xr.panel(reopened, 460, 87);
    await page.keyboard.press("Enter");
    await expectPainted(xr, reopened, "echo: retained");
    expect(
      (await received(request, server, a)).filter(
        (text) => text === "retained",
      ),
    ).toHaveLength(1);

    // Drop only this browser's transport while both ACP agents keep producing.
    await reconnect.drop();
    await turn(remoteA, "A_DURING_OFFLINE");
    await turn(remoteB, "B_DURING_OFFLINE");
    expect(await xr.text(a1Surface.id)).not.toContain("A_DURING_OFFLINE");
    reconnect.release();
    await expectPainted(xr, reopened, "echo: A_DURING_OFFLINE");
    await expectPainted(xr, b1, "echo: B_DURING_OFFLINE");
    const resumedSurface = await viewSurface(xr, reopened);
    expect(
      (await xr.text(resumedSurface.id)).match(/echo: A_DURING_OFFLINE/g),
    ).toHaveLength(1);
    await xr.unobserve("XR A");
    expect((await xr.layouts()).some((view) => view.sessionId === a)).toBe(
      false,
    );
    await turn(remoteB, "B_AFTER_UNOBSERVE");
    await expectPainted(xr, b1, "echo: B_AFTER_UNOBSERVE");
    expect((await (await request.get(`/api/sessions/${a}`)).json()).state).toBe(
      "active",
    );
    await xr.menu("XR A");
    await expectPainted(xr, await chatLayout(xr, a), "echo: A_DURING_OFFLINE");
    // Removing an observed 2D member does not unmount its existing owner.
    await xr.unobserve("XR grid controller");
    await expect(page.locator(".split-terminal")).toHaveCount(1);
    expect(await grid()).toEqual(initialGrid);
    await xr.menu("退出 VR");
    await expect(
      page.getByRole("button", { name: "进入 VR", exact: true }),
    ).toBeEnabled();
    expect(page.url()).toBe(initialRoute);
    expect(await active2d(page)).toBe(initialActive);
    await expect(page.locator(".split-terminal")).toHaveCount(1);
    await runCommand(controller, "printf 'XR_%s_CONTINUITY\\n' AFTER");
    await expectShellOutput(request, shell, "XR_AFTER_CONTINUITY");
    const buffer = await (
      await request.get(`/api/sessions/${shell}/buffer`)
    ).text();
    expect(buffer.match(/XR_AFTER_CONTINUITY/g)).toHaveLength(1);
    expect(buffer).not.toContain("mustnotroute");
    expect(await grid()).toEqual(initialGrid);
    expect(
      (await (await request.get("/api/sessions")).json())
        .map((item: { id: string }) => item.id)
        .sort(),
    ).toEqual([a, b, shell].sort());
    await xr.enter();
    expect(await visibleChat(xr, a)).toHaveLength(1);
    await expectPainted(xr, await chatLayout(xr, a), "echo: A_DURING_OFFLINE");
    await xr.menu("退出 VR");
  } finally {
    reconnect.release();
    await other.close();
  }
});

// Cross-device permission resolution must invalidate an already-open XR
// confirmation, including the other copy of the same canonical conversation.
test("IWER permission confirmation expires when another device answers first", async ({
  page,
  request,
  browser,
  server,
  workDir,
  xr,
}, info) => {
  test.skip(!!info.project.use.isMobile, "Immersive desktop runtime");
  test.setTimeout(60_000);
  const a = await createChat(request, workDir, "XR permission", true);
  const mode = await request.put(`/api/sessions/${a}/chat/permission-mode`, {
    data: { clientRequestId: "xr-permission-ask", mode: "ask" },
  });
  expect(mode.status(), await mode.text()).toBe(204);
  const requestIds = new Set<string>();
  page.on("websocket", (socket) => {
    if (!new URL(socket.url()).pathname.endsWith("/chat")) return;
    socket.on("framereceived", ({ payload }) => {
      const frame = JSON.parse(String(payload));
      for (const item of frame.state?.requests ?? [])
        requestIds.add(item.requestId);
    });
  });
  const other = await browser.newContext({ baseURL: server.baseURL });
  try {
    const remote = await other.newPage();
    await remote.goto(`/s/${a}`);
    await page.goto(`/s/${a}`);
    await expect(
      page.getByRole("textbox", { name: "消息", exact: true }),
    ).toBeEditable();
    await xr.enter();
    const a1 = await chatLayout(xr, a);
    await xr.panel(a1, 700, 736);
    await expect.poll(async () => (await visibleChat(xr, a)).length).toBe(2);
    const a2 = (await visibleChat(xr, a)).find((view) => view.id !== a1.id)!;
    await xr.panel(a2, 460, 87);
    await page.keyboard.type("needsoneauthorization");
    await page.keyboard.press("Enter");
    await expectPainted(xr, a1, "等待处理：Read file");
    const surface = await viewSurface(xr, a1);
    const allow = (await xr.surface(surface.id))!.text.find(
      (line) => line.text === "Allow" || line.text === "Allow once",
    );
    expect(allow, "Painted current permission option").toBeTruthy();
    await xr.panel(a1, allow!.x + 20, allow!.y);
    await expectPainted(xr, a1, "确认操作");
    await remote
      .getByRole("region", { name: "等待授权", exact: true })
      .getByRole("button", { name: "Reject", exact: true })
      .click();
    await expectPainted(xr, a1, "echo: needsoneauthorization (rejected)");
    await expectPainted(xr, a2, "echo: needsoneauthorization (rejected)");
    // The old confirm coordinate cannot authorize a request after it is removed.
    await xr.panel(a1, 280, 465);
    await expect.poll(() => xr.text(surface.id)).not.toContain("确认执行");
    expect(
      (await received(request, server, a)).filter(
        (text) => text === "needsoneauthorization",
      ),
    ).toHaveLength(1);
    expect(requestIds.size).toBe(1);
    const late = await request.post(
      `/api/sessions/${a}/chat/requests/${[...requestIds][0]}`,
      {
        data: {
          clientRequestId: "xr-late-answer",
          outcome: "selected",
          optionId: "allow",
        },
      },
    );
    expect(late.status()).toBe(409);
    expect((await late.json()).reason).toBe("answered");
    await xr.menu("退出 VR");
  } finally {
    await other.close();
  }
});

test("IWER keeps B live when the entry owner is permanently removed", async ({
  page,
  request,
  browser,
  server,
  workDir,
  xr,
}, info) => {
  test.skip(!!info.project.use.isMobile, "Immersive desktop runtime");
  test.setTimeout(60_000);
  const a = await createChat(request, workDir, "XR deleted entry");
  const b = await createChat(request, workDir, "XR surviving B");
  const other = await browser.newContext({ baseURL: server.baseURL });
  try {
    const remote = await other.newPage();
    await remote.goto(`/s/${b}`);
    await turn(remote, "B_BEFORE_DELETE");
    await page.goto(`/s/${a}`);
    await expect(
      page.getByRole("textbox", { name: "消息", exact: true }),
    ).toBeEditable();
    await xr.enter();
    const a1 = await chatLayout(xr, a);
    await xr.menu("XR surviving B");
    const b1 = await chatLayout(xr, b);
    await expectPainted(xr, b1, "B_BEFORE_DELETE");
    await xr.panel(a1, 460, 87);
    // Plain DELETE only archives a hybrid session and leaves its route recoverable.
    // forget=1 waits for closure, removes the shell and deletes the archive record.
    expect((await request.delete(`/api/sessions/${a}?forget=1`)).status()).toBe(
      204,
    );
    expect((await request.get(`/api/sessions/${a}`)).status()).toBe(404);
    await expect
      .poll(async () =>
        (await xr.layouts()).some((view) => view.sessionId === a),
      )
      .toBe(false);
    await page.keyboard.type("closedtarget");
    await page.keyboard.press("Enter");
    await turn(remote, "B_AFTER_DELETE");
    await expectPainted(xr, b1, "echo: B_AFTER_DELETE");
    expect(await received(request, server, b)).not.toContain("closedtarget");
    await xr.menu("退出 VR");
    await expect(
      page.getByRole("button", { name: "退出 VR", exact: true }),
    ).toHaveCount(0);
    await expect(page).not.toHaveURL(new RegExp(`/s/${a}$`));
    await expect(page.locator(`[data-session-id="${a}"]`)).toHaveCount(0);
    expect((await (await request.get(`/api/sessions/${b}`)).json()).state).toBe(
      "active",
    );
  } finally {
    await other.close();
  }
});

test("IWER keeps an archived entry read-only before and after the live snapshot", async ({
  page,
  request,
  workDir,
  xr,
}, info) => {
  test.skip(
    !!info.project.use.isMobile,
    "Immersive software-GL desktop runtime; real headset remains separate acceptance",
  );
  const a = await createChat(request, workDir, "XR archived entry");
  const b = await createChat(request, workDir, "XR live peer");
  expect((await request.delete(`/api/sessions/${a}`)).ok()).toBe(true);
  await expect
    .poll(
      async () =>
        (await (await request.get(`/api/sessions/${a}`)).json()).state,
    )
    .toBe("closed");
  const snapshot = Promise.withResolvers<void>();
  await page.routeWebSocket(/\/ws\/sessions(?:\?|$)/, async (socket) => {
    await snapshot.promise;
    socket.connectToServer();
  });
  try {
    await page.goto(`/s/${a}`);
    const recovery = page.getByRole("status", { name: "恢复会话" });
    await expect(recovery).toBeVisible();
    await xr.enter();
    const initial = await chatLayout(xr, a);
    await expectPainted(xr, initial, "只读");
    snapshot.resolve();
    await xr.menu("XR live peer");
    await chatLayout(xr, b);
    await expectPainted(xr, initial, "恢复会话");
    expect(await active2d(page)).toBe(a);
    await expect(page.locator(".split-terminal")).toHaveCount(1);
    await xr.menu("退出 VR");
    await expect(
      page.getByRole("button", { name: "进入 VR", exact: true }),
    ).toBeEnabled();
    await xr.enter();
    const resumed = await chatLayout(xr, a);
    const surfaceId = await expectPainted(xr, resumed, "只读");
    const surface = await xr.surface(surfaceId);
    const action = surface?.text.find(
      (line) => line.text === "返回 2D 恢复会话",
    );
    expect(action).toBeDefined();
    await xr.panel(resumed, action!.x + 10, action!.y);
    await expect(
      page.getByRole("button", { name: "进入 VR", exact: true }),
    ).toBeEnabled();
    await expect(recovery).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/s/${a}$`));
    expect((await (await request.get(`/api/sessions/${a}`)).json()).state).toBe(
      "closed",
    );
    expect((await (await request.get(`/api/sessions/${b}`)).json()).state).toBe(
      "active",
    );
  } finally {
    snapshot.resolve();
  }
});
