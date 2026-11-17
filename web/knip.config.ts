import type { KnipConfig } from "knip";
import projectUI from "./ui-project-components.json" with { type: "json" };

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
    `src/lib/components/ui/!(${projectUI.join("|")})/**`,
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
