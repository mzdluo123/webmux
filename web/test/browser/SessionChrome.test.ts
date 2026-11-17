import { afterEach, expect, test } from "vitest";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./SessionChromeHarness.svelte";

afterEach(cleanup);

interface SessionChromeApi {
  setEditing: (value: boolean) => void;
  editing: () => boolean;
  releases: () => number;
}

const settled = () =>
  new Promise<void>((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  );

async function open(dismissed: boolean) {
  let api: SessionChromeApi | undefined;
  const view = await render(Harness, {
    dismissed,
    expose: (value: SessionChromeApi) => {
      api = value;
    },
  });
  await settled();
  return { api: api!, view };
}

// An IME dismissal while the chat input holds focus releases editing once.
// The flag then latches until the keyboard reopens; on fixed shells (tablets,
// standalone apps, split panes, code panel) it must not keep rejecting every
// later tap on the message box.
test("a latched IME dismissal releases chat editing once, not every focus", async () => {
  const { api, view } = await open(false);
  api.setEditing(true);
  await settled();
  expect(api.editing()).toBe(true);
  expect(api.releases()).toBe(0);

  // The soft keyboard opens and is dismissed: the focused editing ends.
  await view.rerender({ dismissed: true });
  await settled();
  expect(api.releases()).toBe(1);
  expect(api.editing()).toBe(false);

  // The flag stays latched while the page remains a fixed shell, yet the next
  // tap on the input is a fresh deliberate focus and must survive.
  api.setEditing(true);
  await settled();
  expect(api.releases()).toBe(1);
  expect(api.editing()).toBe(true);

  // The IME reopening clears the latch; a later dismissal releases again.
  await view.rerender({ dismissed: false });
  await view.rerender({ dismissed: true });
  await settled();
  expect(api.releases()).toBe(2);
  expect(api.editing()).toBe(false);
});

// A dismissal observed before the chat was ever editing (latched from an
// earlier view or a remounted pane) is not a reason to drop the first focus.
test("a dismissal latched before chat editing does not drop the first focus", async () => {
  const { api } = await open(true);
  api.setEditing(true);
  await settled();
  expect(api.releases()).toBe(0);
  expect(api.editing()).toBe(true);
});
