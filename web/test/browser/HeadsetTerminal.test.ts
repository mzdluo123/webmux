import { afterEach, expect, test } from "vitest";
import { userEvent } from "vitest/browser";
import { headsetMode } from "../../src/hooks/shared/headset-mode.svelte";
import {
  blurAndTrackFocus,
  host,
  inputText,
  installTerminalFixture,
  openTerminalApi,
} from "./terminal-fixture";

installTerminalFixture();
afterEach(() => {
  headsetMode.enabled = false;
});

test("controller mouse activation leaves keyboard closed while explicit input still sends once", async () => {
  headsetMode.enabled = true;
  const { api, socket } = await openTerminalApi();
  const { textarea, focused } = blurAndTrackFocus();
  const screen = host().querySelector<HTMLElement>(".xterm-screen")!;
  const bounds = screen.getBoundingClientRect();
  const select = () =>
    screen.dispatchEvent(
      new MouseEvent("mousedown", {
        bubbles: true,
        cancelable: true,
        button: 0,
        clientX: bounds.left + 10,
        clientY: bounds.top + 10,
      }),
    );

  expect(select()).toBe(false);
  expect(document.activeElement).not.toBe(textarea);
  expect(focused).not.toHaveBeenCalled();
  expect(inputText(socket)).toBe("");

  api.keyboard();
  expect(document.activeElement).toBe(textarea);
  await userEvent.keyboard("echo headset{Enter}");
  await expect.poll(() => inputText(socket)).toBe("echo headset\r");
  expect(focused).toHaveBeenCalledTimes(1);
  api.keyboard();
  expect(document.activeElement).not.toBe(textarea);

  // The explicit presentation must not change ordinary desktop mouse focus.
  headsetMode.enabled = false;
  select();
  expect(document.activeElement).toBe(textarea);
  expect(focused).toHaveBeenCalledTimes(2);
  expect(inputText(socket)).toBe("echo headset\r");
});
