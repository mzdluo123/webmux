import type { KnipConfig } from "knip";
import generatedUI from "./ui-generated-files.json" with { type: "json" };

export default {
  entry: ["scripts/*.{ts,mjs}"],
  vitest: {
    config: ["vitest.config.ts", "vitest.browser.config.ts"],
  },
  project: [
    "src/**/*.{ts,svelte,css}",
    "scripts/**/*.{ts,mjs}",
    "test/**/*.ts",
  ],
  ignore: [
    ...generatedUI.map((file) => `src/lib/components/ui/${file}`),
    "src/lib/paraglide/**",
  ],
  ignoreDependencies: [
    "@inlang/plugin-message-format",
    "@typescript/native",
    "shadcn-svelte",
  ],
  rules: {
    files: "error",
    duplicates: "error",
    dependencies: "error",
    exports: "error",
    types: "error",
  },
} satisfies KnipConfig;
