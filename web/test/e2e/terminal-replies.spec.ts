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

for (const theme of ["light", "dark"] as const) {
  test(`a new TUI learns the ${theme} theme even if it probes before the browser attaches`, async ({
    page,
    request,
    workDir,
  }) => {
    await page.addInitScript(
      (value) => localStorage.setItem("webmux.theme", value),
      theme,
    );
    // Like omp, fence capability queries with DA1. A theme notification
    // triggers a fresh OSC 11 query only after the startup probe has finished.
    const program = String.raw`stty raw -echo
exec perl - 3<&0 <<'PERL'
open my $tty, "<&=3" or die $!;
$| = 1;
print "\e[c\e]11;?\a\e[0c\e[?2031h";
my ($fences, $live, $reply) = (0, 0, "");
while (sysread($tty, my $byte, 1)) {
    $reply .= $byte;
    if ($reply =~ /\e\[\?[\d;]+c$/) {
        ++$fences;
    } elsif ($reply =~ /\e\[\?997;[12]n$/) {
        print "\e]11;?\a" if $fences == 2 && !$live++;
    } elsif ($reply =~ m{\e\]11;rgb:([0-9a-f/]+)\e\\$}) {
        print "\r\n", $live ? "LIVE_BG=$1" : "UNEXPECTED_STARTUP_COLOR", "\r\n";
    } else {
        next;
    }
    $reply = "";
}
PERL`;
    const created = await request.post("/api/sessions", {
      data: { cwd: workDir, cmd: ["/bin/bash", "-c", program] },
    });
    expect(created.status()).toBe(201);
    const { id } = (await created.json()).session;
    await expect
      .poll(async () =>
        (await request.get(`/api/sessions/${id}/buffer`)).text(),
      )
      .toContain("\x1b[?2031h");

    await page.goto(`/s/${id}`);
    const rgb = theme === "light" ? "ffff/ffff/ffff" : "0d0d/1111/1717";
    await expectTerminalLine(page, `LIVE_BG=${rgb}`);
    await expect(page.locator(".xterm-rows")).not.toContainText(
      "UNEXPECTED_STARTUP_COLOR",
    );
  });
}
