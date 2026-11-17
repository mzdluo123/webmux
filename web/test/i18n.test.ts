import { globSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, test } from "vite-plus/test";
import { CodeNameSchema } from "../src/contracts/code-schemas";
import {
  QUICK_COMMAND_PRESETS,
  commandLabel,
  expandInitInstruction,
  quickCommandNameError,
} from "../src/domains/settings/quick-commands";
import { LAUNCH_ATTENTION_MODE_OPTIONS } from "../src/domains/settings/launch-configs";
import {
  initializeLocale,
  locale,
  resolveSystemLocale,
  type LocalePreference,
  type SupportedLocale,
} from "../src/lib/locale";
import * as m from "../src/lib/paraglide/messages.js";

const root = fileURLToPath(new URL("..", import.meta.url));
const PLACEHOLDER = /\{([^{}]+)\}/g;
const CJK = /[\u4e00-\u9fff]/;

function readCatalog(name: SupportedLocale): Record<string, unknown> {
  const raw = JSON.parse(
    readFileSync(`${root}/messages/${name}.json`, "utf8"),
  ) as Record<string, unknown>;
  const { $schema: _schema, ...entries } = raw;
  return entries;
}

function stripComments(source: string): string {
  let out = "";
  let i = 0;
  const n = source.length;
  let state: "code" | "line" | "block" | "html" | '"' | "'" | "`" = "code";
  while (i < n) {
    const ch = source[i]!;
    const next = source[i + 1];
    if (state === "code") {
      if (ch === "/" && next === "/") {
        out += "  ";
        i += 2;
        state = "line";
      } else if (ch === "/" && next === "*") {
        out += "  ";
        i += 2;
        state = "block";
      } else if (source.startsWith("<!--", i)) {
        out += "    ";
        i += 4;
        state = "html";
      } else if (ch === '"' || ch === "'" || ch === "`") {
        out += ch;
        i++;
        state = ch;
      } else {
        out += ch;
        i++;
      }
    } else if (state === "line") {
      if (ch === "\n") {
        out += "\n";
        i++;
        state = "code";
      } else {
        out += " ";
        i++;
      }
    } else if (state === "block") {
      if (ch === "*" && next === "/") {
        out += "  ";
        i += 2;
        state = "code";
      } else {
        out += ch === "\n" ? "\n" : " ";
        i++;
      }
    } else if (state === "html") {
      if (source.startsWith("-->", i)) {
        out += "   ";
        i += 3;
        state = "code";
      } else {
        out += ch === "\n" ? "\n" : " ";
        i++;
      }
    } else if (ch === "\\") {
      out += ch + (next ?? "");
      i += 2;
    } else if (ch === state) {
      out += ch;
      i++;
      state = "code";
    } else {
      out += ch;
      i++;
    }
  }
  return out;
}

function catalogPatterns(value: unknown): string[] {
  if (typeof value === "string") return [value];
  if (Array.isArray(value)) {
    return value.flatMap((entry) => {
      if (
        entry &&
        typeof entry === "object" &&
        "match" in entry &&
        entry.match &&
        typeof entry.match === "object"
      ) {
        return Object.values(entry.match as Record<string, string>);
      }
      return [];
    });
  }
  return [];
}

afterEach(() => {
  locale.setPreference("zh-CN");
});

describe("i18n locale resolution, dynamic reactivity, and catalog parity", () => {
  test("resolves browser language lists with priority and fallback", () => {
    expect(resolveSystemLocale(["zh-CN", "en-US"])).toBe("zh-CN");
    expect(resolveSystemLocale(["zh-TW", "en"])).toBe("zh-CN");
    expect(resolveSystemLocale(["en-US", "zh-CN"])).toBe("en");
    expect(resolveSystemLocale(["en-GB"])).toBe("en");
    expect(resolveSystemLocale(["ja-JP", "zh-Hans"])).toBe("zh-CN");
    expect(resolveSystemLocale(["fr-FR"])).toBe("en");
    expect(resolveSystemLocale([])).toBe("zh-CN");
  });

  test("switches Paraglide messages, Zod errors, and domain presets reactively", () => {
    initializeLocale();
    const choices: LocalePreference[] = ["zh-CN", "en", "system"];
    expect(choices).toContain(locale.preference);

    locale.setPreference("zh-CN");
    expect(locale.current).toBe("zh-CN");
    expect(m.locale_label()).toBe("语言");
    expect(m.locale_follow_system()).toBe("跟随系统");
    expect(CodeNameSchema.safeParse(".git").error?.issues[0]?.message).toBe(
      ".git 为保留名称",
    );
    expect(quickCommandNameError("bad{name}", [])).toBe(
      "名称不能包含 { }或换行",
    );
    expect(QUICK_COMMAND_PRESETS[0]?.name).toBe("恢复会话");
    expect(commandLabel(QUICK_COMMAND_PRESETS[0]!)).toBe("恢复会话");
    expect(LAUNCH_ATTENTION_MODE_OPTIONS[0]?.label).toBe("全部提醒");

    locale.setPreference("en");
    expect(locale.current).toBe("en");
    expect(m.locale_label()).toBe("Language");
    expect(m.locale_follow_system()).toBe("Follow system");
    const enGitIssue =
      CodeNameSchema.safeParse(".git").error?.issues[0]?.message ?? "";
    expect(enGitIssue).toMatch(/\.git/i);
    expect(CJK.test(enGitIssue)).toBe(false);
    const enCmdErr = quickCommandNameError("bad{name}", []);
    expect(enCmdErr).toContain("{ }");
    expect(CJK.test(enCmdErr)).toBe(false);
    // Stored reference key stays locale-independent while display label localizes.
    expect(QUICK_COMMAND_PRESETS[0]?.name).toBe("恢复会话");
    expect(commandLabel(QUICK_COMMAND_PRESETS[0]!)).toBe("Resume session");
    const clonedInEn = QUICK_COMMAND_PRESETS.map((preset) => ({ ...preset }));
    expect(
      expandInitInstruction("{{WebMux 工具}}", clonedInEn).missing,
    ).toEqual([]);
    expect(CJK.test(LAUNCH_ATTENTION_MODE_OPTIONS[0]?.label ?? "")).toBe(false);

    // English pluralization variants handle singular and plural counts naturally.
    expect(m.s8_details_total_sessions({ count: 1 })).toBe("1 session");
    expect(m.s8_details_total_sessions({ count: 2 })).toBe("2 sessions");
    expect(m.s7_file_count_badge({ count: "1" })).toBe("1 file");
    expect(m.s7_file_count_badge({ count: "3" })).toBe("3 files");
    expect(m.s2_sys_preset_count({ count: 1 })).toBe("1 item");
    expect(m.s2_sys_preset_count({ count: 4 })).toBe("4 items");
    expect(m.s1_node_sessions_count({ label: "webmux", count: 1 })).toBe(
      "webmux, 1 session",
    );
    expect(m.s1_node_sessions_count({ label: "webmux", count: 2 })).toBe(
      "webmux, 2 sessions",
    );
  });

  test("zh-CN and en catalogs define identical keys and matching placeholders", () => {
    const zh = readCatalog("zh-CN");
    const en = readCatalog("en");
    expect(Object.keys(en).sort()).toEqual(Object.keys(zh).sort());
    expect(Object.keys(zh).length).toBeGreaterThan(4000);

    const mismatches: string[] = [];
    for (const [key, zhVal] of Object.entries(zh)) {
      const enVal = en[key];
      const zhPatterns = catalogPatterns(zhVal);
      const enPatterns = catalogPatterns(enVal);
      expect(zhPatterns.length, `zh-CN.${key} has no patterns`).toBeGreaterThan(
        0,
      );
      expect(enPatterns.length, `en.${key} has no patterns`).toBeGreaterThan(0);

      for (const pattern of zhPatterns) {
        expect(pattern.trim().length, `zh-CN.${key} is empty`).toBeGreaterThan(
          0,
        );
      }
      const zhParams = [
        ...new Set([...zhPatterns[0]!.matchAll(PLACEHOLDER)].map((i) => i[1])),
      ].sort();
      for (const pattern of enPatterns) {
        expect(pattern.trim().length, `en.${key} is empty`).toBeGreaterThan(0);
        const enParams = [
          ...new Set([...pattern.matchAll(PLACEHOLDER)].map((i) => i[1])),
        ].sort();
        if (zhParams.join(",") !== enParams.join(",")) {
          mismatches.push(
            `${key}: zh=[${zhParams.join(",")}] vs en=[${enParams.join(",")}]`,
          );
        }
      }
    }
    expect(mismatches).toEqual([]);
  });

  test("production source files contain no untranslated hardcoded CJK strings", () => {
    const prodFiles = globSync("src/**/*.{ts,svelte}", { cwd: root })
      .map((file) => file.replaceAll("\\", "/"))
      .filter(
        (file) =>
          !file.startsWith("src/lib/paraglide/") &&
          file !== "src/components/shared/LocaleChoice.svelte",
      )
      .sort();

    const violations: string[] = [];
    for (const file of prodFiles) {
      const raw = readFileSync(`${root}/${file}`, "utf8");
      const stripped = stripComments(raw);
      expect(
        stripped.split("\n").length,
        `${file} line count changed during comment stripping`,
      ).toBe(raw.split("\n").length);
      for (const [idx, line] of stripped.split("\n").entries()) {
        if (CJK.test(line)) {
          violations.push(`${file}:${idx + 1}: ${line.trim()}`);
        }
      }
    }
    expect(
      violations,
      "Extract UI strings into web/messages/{zh-CN,en}.json via $lib/paraglide/messages.js.",
    ).toEqual([]);
  });

  test("production source files import named { m } without shadowing so Vite rewrites virtual message imports", () => {
    const prodFiles = globSync("src/**/*.{ts,svelte}", { cwd: root })
      .map((file) => file.replaceAll("\\", "/"))
      .filter((file) => !file.startsWith("src/lib/paraglide/"))
      .sort();

    const violations: string[] = [];
    for (const file of prodFiles) {
      const raw = readFileSync(`${root}/${file}`, "utf8");
      if (
        /import\s+\*\s+as\s+\w+\s+from\s+["'][^"']*paraglide\/messages/.test(
          raw,
        )
      ) {
        violations.push(
          `${file}: namespace import prevents virtual message rewriting`,
        );
        continue;
      }
      if (!raw.includes("$lib/paraglide/messages.js")) continue;
      // Paraglide's AST rewriter is lexical-scope agnostic: any local variable
      // or parameter named `m` marks `m` unsafe and pulls in all 4,231 messages.
      const stripped = stripComments(raw).replace(
        /import\s+\{\s*m\s*\}\s+from\s+"\$lib\/paraglide\/messages\.js";/g,
        "",
      );
      for (const match of stripped.matchAll(/(?<![.\w$-])m(?![\w$])/g)) {
        const idx = match.index ?? 0;
        const after = stripped.slice(idx, idx + 80);
        if (/^m\.[a-z][a-z0-9_]*\s*\(/.test(after)) continue;
        // Allow HTML/Tailwind attributes like `m-0` or template literals `m:${...}`.
        if (/^m[-:]/.test(after)) continue;
        const line = raw.slice(0, idx).split("\n").length;
        violations.push(
          `${file}:${line}: non-message identifier \`m\` blocks tree-shaking`,
        );
      }
    }
    expect(
      violations,
      'Use `import { m } from "$lib/paraglide/messages.js"` and do not shadow `m` in the same file.',
    ).toEqual([]);
  });
});
