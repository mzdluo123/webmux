import { readFile } from "node:fs/promises";
import { join } from "node:path";
import {
  test,
  expect,
  createShell,
  runCommand,
  expectConnected,
  expectTerminalLine,
  quote,
} from "./fixtures";

test("reopening suppresses historical device replies and preserves live replies", async ({
  page,
  workDir,
}) => {
  await createShell(page, "Replay device replies", workDir);
  const path = join(workDir, "device-replies");
  // Raw input records exactly what reaches the program. ! requests a new
  // live query; all other bytes, including automatic responses, are retained.
  await runCommand(
    page,
    `stty raw -echo; : > ${quote(path)}; query() { printf '\\033]10;?\\007\\033]11;?\\007'; }; query; printf 'QUERY_READY\\r\\n'; while IFS= read -r -n 1 byte; do if [ "$byte" = '!' ]; then query; else printf '%s' "$byte" >> ${quote(path)}; fi; done`,
  );
  await expectTerminalLine(page, "QUERY_READY");
  await expect
    .poll(async () => (await readFile(path, "utf8")).replaceAll("\x1b", "ESC"))
    .toMatch(/^ESC\]10;rgb:[0-9a-f/]+ESC\\ESC\]11;rgb:[0-9a-f/]+ESC\\$/);
  const initial = await readFile(path, "utf8");

  for (let reopen = 0; reopen < 2; reopen++) {
    await page.reload();
    await expectConnected(page);
    await expect(page.locator(".restore-overlay")).toBeHidden();
    await expectTerminalLine(page, "QUERY_READY");
    // A real input byte fences earlier replay replies through the same PTY.
    await page.locator(".xterm-helper-textarea").pressSequentially("~");
    await expect
      .poll(() => readFile(path, "utf8"))
      .toBe(initial + "~".repeat(reopen + 1));
  }

  await page.locator(".xterm-helper-textarea").pressSequentially("!");
  await expect
    .poll(() => readFile(path, "utf8"))
    .toBe(initial + "~~" + initial);
});
