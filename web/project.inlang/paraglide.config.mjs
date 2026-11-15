import { defineConfig } from "@inlang/paraglide-js";

export default defineConfig({
  outdir: "./src/lib/paraglide",
  strategy: ["localStorage", "preferredLanguage", "baseLocale"],
  localStorageKey: "webmux.locale",
  outputStructure: "locale-modules",
  emitTsDeclarations: true,
  emitReadme: false,
  emitGitIgnore: true,
  emitPrettierIgnore: true,
});
