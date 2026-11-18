import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import TimelineItem from "../../src/components/chat/TimelineItem.svelte";

afterEach(async () => {
  await cleanup();
  vi.unstubAllGlobals();
});

test.each(["loaded", "unavailable"])(
  "keeps the next message in place when a history image is %s",
  async (state) => {
    const response = Promise.withResolvers<Response>();
    const fetch = vi.fn(() => response.promise);
    vi.stubGlobal("fetch", fetch);
    render(TimelineItem, {
      entry: {
        type: "message",
        id: "history",
        role: "user",
        text: "Earlier message",
        attachments: [
          {
            path: "/uploads/photo.svg",
            name: "photo.svg",
            kind: "image",
            mimeType: "image/svg+xml",
          },
        ],
      },
      waiting: [],
      sessionId: "fixture",
    });
    render(TimelineItem, {
      entry: {
        type: "message",
        id: "next",
        role: "agent",
        text: "Next message",
      },
      waiting: [],
    });
    const row = page.getByRole("listitem");
    const next = page.getByText("Next message");
    await expect.element(next).toBeVisible();
    expect(fetch).toHaveBeenCalledOnce();
    await document.fonts.ready;
    const before = row.element().getBoundingClientRect().toJSON();
    const nextTop = next.element().getBoundingClientRect().top;

    const unavailable = new Response("missing", { status: 404 });
    const readError = vi.spyOn(unavailable, "json");
    response.resolve(
      state === "loaded"
        ? new Response(
            '<svg xmlns="http://www.w3.org/2000/svg" width="60" height="120"/>',
            {
              headers: { "Content-Type": "image/svg+xml" },
            },
          )
        : unavailable,
    );
    if (state === "loaded") {
      const image = page.getByRole("img", { name: "photo.svg" });
      await expect.element(image).toBeVisible();
      await (image.element() as HTMLImageElement).decode();
    } else {
      await vi.waitFor(() => expect(readError).toHaveBeenCalledOnce());
      await readError.mock.results[0]!.value.catch(() => null);
      await new Promise<void>((resolve) =>
        requestAnimationFrame(() => resolve()),
      );
      expect(row.element().querySelector("svg")).not.toBeNull();
    }
    expect(row.element().getBoundingClientRect().toJSON()).toEqual(before);
    expect(next.element().getBoundingClientRect().top).toBe(nextTop);
  },
);
