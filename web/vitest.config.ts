import { defineConfig } from "vite-plus";

export const sharedCoverage = {
  provider: "v8" as const,
  reportOnFailure: true,
  include: ["src/**/*.{ts,svelte}"],
  exclude: [
    "src/**/*.d.ts",
    "src/main.ts",
    "src/icons/**",
    "src/lib/components/ui/**",
    "src/lib/paraglide/**",
  ],
};

export default defineConfig({
  resolve: {
    alias: { $lib: new URL("./src/lib", import.meta.url).pathname },
  },
  test: {
    environment: "node",
    include: ["test/*.test.ts"],
    setupFiles: ["./test/setup-unit.ts"],
    coverage: {
      ...sharedCoverage,
      include: ["src/**/*.ts"],
      exclude: [...sharedCoverage.exclude, "src/**/*.svelte.ts"],
      reportsDirectory: ".vitest/coverage/unit",
      reporter: ["text-summary", "json", "json-summary", "html"],
    },
  },
});
