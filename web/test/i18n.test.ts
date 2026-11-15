import { globSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, test } from "vite-plus/test";
import { CodeNameSchema } from "../src/contracts/code-schemas";
import {
  QUICK_COMMAND_PRESETS,
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
  return source
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|\s)\/\/.*$/gm, "$1");
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
    expect(CJK.test(QUICK_COMMAND_PRESETS[0]?.name ?? "")).toBe(false);
    expect(CJK.test(LAUNCH_ATTENTION_MODE_OPTIONS[0]?.label ?? "")).toBe(false);
  });

  test("zh-CN and en catalogs define identical keys and matching placeholders", () => {
    const zh = readCatalog("zh-CN");
    const en = readCatalog("en");
    expect(Object.keys(en).sort()).toEqual(Object.keys(zh).sort());
    expect(Object.keys(zh).length).toBeGreaterThan(4000);

    const mismatches: string[] = [];
    for (const [key, zhVal] of Object.entries(zh)) {
      const enVal = en[key];
      if (typeof zhVal === "string" && typeof enVal === "string") {
        expect(zhVal.trim().length, `zh-CN.${key} is empty`).toBeGreaterThan(0);
        expect(enVal.trim().length, `en.${key} is empty`).toBeGreaterThan(0);
        const zhParams = [...zhVal.matchAll(PLACEHOLDER)]
          .map((item) => item[1])
          .sort();
        const enParams = [...enVal.matchAll(PLACEHOLDER)]
          .map((item) => item[1])
          .sort();
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
      const stripped = stripComments(readFileSync(`${root}/${file}`, "utf8"));
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

  test("production source files import named { m } so Vite rewrites virtual message imports", () => {
    const prodFiles = globSync("src/**/*.{ts,svelte}", { cwd: root })
      .map((file) => file.replaceAll("\\", "/"))
      .filter((file) => !file.startsWith("src/lib/paraglide/"))
      .sort();

    const namespaceImports = prodFiles.filter((file) =>
      /import\s+\*\s+as\s+\w+\s+from\s+["'][^"']*paraglide\/messages/.test(
        readFileSync(`${root}/${file}`, "utf8"),
      ),
    );
    expect(
      namespaceImports,
      'Use `import { m } from "$lib/paraglide/messages.js"` so Paraglide only loads referenced messages.',
    ).toEqual([]);
  });
});
