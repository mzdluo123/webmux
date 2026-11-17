import { defineConfig } from "vite-plus";

// A standalone static server: importing vite.config.ts would inherit its API proxy.
export default defineConfig({
  build: { outDir: "storybook-static" },
  preview: { host: "127.0.0.1", strictPort: true },
});
