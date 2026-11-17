import type { Preview } from "@storybook/svelte-vite";
import { locale } from "../src/lib/locale";
import "../src/styles.css";

const preview: Preview = {
  parameters: {
    layout: "fullscreen",
    options: { storySort: { order: ["Design system", "Workspace", "UI"] } },
  },
  initialGlobals: { theme: "light", locale: "zh-CN" },
  globalTypes: {
    theme: {
      description: "Application theme",
      toolbar: { icon: "circlehollow", items: ["light", "dark"] },
    },
    locale: {
      description: "Interface language",
      toolbar: { icon: "globe", items: ["zh-CN", "en"] },
    },
  },
  decorators: [
    (story, { globals }) => {
      document.documentElement.classList.toggle(
        "dark",
        globals.theme === "dark",
      );
      locale.setPreference(globals.locale === "en" ? "en" : "zh-CN");
      return story();
    },
  ],
};

export default preview;
