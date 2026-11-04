import { expect, test } from "vitest";
import { commitChoices, relativeTime } from "../src/commitChoices";

const now = Date.parse("2026-11-04T12:00:00Z");
const commit = (hash: string, subject: string, committedAt: string) => ({
  hash,
  subject,
  author: "dev",
  committedAt,
});
const a = "a".repeat(40);
const b = "b".repeat(40);

test("relative time picks the largest whole unit and tolerates bad dates", () => {
  expect(relativeTime("2026-11-04T09:00:00Z", now)).toBe("3小时前");
  expect(relativeTime("2026-11-02T12:00:00Z", now)).toBe("前天");
  expect(relativeTime("2026-11-04T11:59:40Z", now)).toBe("刚刚");
  expect(relativeTime("not a date", now)).toBe("");
});

test("rows show short hash, title and age; only the first is tagged", () => {
  const rows = commitChoices(
    [
      commit(a, "修复重连", "2026-11-04T09:00:00Z"),
      commit(b, "", "2026-11-02T12:00:00Z"),
    ],
    { tip: "最新", base: a, baseSubject: "", now },
  );
  expect(rows).toEqual([
    { value: a, label: "最新 · aaaaaaa  修复重连 · 3小时前" },
    { value: b, label: "bbbbbbb  无标题提交 · 前天" },
  ]);
});

test("a base outside the history stays selectable as the first row", () => {
  const c = "c".repeat(40);
  const rows = commitChoices([commit(a, "x", "2026-11-04T09:00:00Z")], {
    tip: "最新",
    base: c,
    baseSubject: "远程提交",
    now,
  });
  expect(rows.map((row) => row.value)).toEqual([c, a]);
  expect(rows[0]?.label).toBe("ccccccc  远程提交");
  expect(
    commitChoices([], { tip: "最新", base: "", baseSubject: "", now }),
  ).toEqual([]);
});
