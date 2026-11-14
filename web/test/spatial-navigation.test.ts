import { expect, test } from "vitest";
import { spatialCandidate } from "../src/platform/spatial-navigation";

const current = { left: 100, right: 150, top: 100, bottom: 150 };

test("directional selection stays in its lane and is deterministic", () => {
  const targets = [
    {
      target: "diagonal",
      rect: { left: 160, right: 210, top: 170, bottom: 220 },
    },
    { target: "right", rect: { left: 180, right: 230, top: 100, bottom: 150 } },
    { target: "left", rect: { left: 20, right: 70, top: 100, bottom: 150 } },
  ];
  expect(spatialCandidate(current, targets, "ArrowRight")).toBe("right");
  expect(spatialCandidate(current, targets, "ArrowLeft")).toBe("left");
  expect(spatialCandidate(current, targets, "ArrowUp")).toBeUndefined();
  expect(spatialCandidate(current, [], "ArrowDown")).toBeUndefined();
});

test("touching center lines are not directional movement and ties retain DOM order", () => {
  const rect = { left: 100, right: 150, top: 170, bottom: 220 };
  expect(
    spatialCandidate(
      current,
      [{ target: "same", rect: current }],
      "ArrowRight",
    ),
  ).toBeUndefined();
  expect(
    spatialCandidate(
      current,
      [
        { target: "first", rect },
        { target: "second", rect },
      ],
      "ArrowDown",
    ),
  ).toBe("first");
});

test("a wide adjacent editor wins over a closer diagonal button center", () => {
  expect(
    spatialCandidate(
      current,
      [
        {
          target: "editor",
          rect: { left: 170, right: 1200, top: 100, bottom: 150 },
        },
        {
          target: "below",
          rect: { left: 130, right: 220, top: 170, bottom: 210 },
        },
      ],
      "ArrowRight",
    ),
  ).toBe("editor");
});
