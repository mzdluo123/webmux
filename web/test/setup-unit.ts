import { beforeEach } from "vite-plus/test";
import { locale } from "../src/lib/locale";

if (typeof navigator !== "undefined") {
  Object.defineProperty(navigator, "languages", {
    value: ["zh-CN"],
    configurable: true,
  });
  Object.defineProperty(navigator, "language", {
    value: "zh-CN",
    configurable: true,
  });
}
locale.setPreference("system");
locale.refresh();

beforeEach(() => {
  if (typeof navigator !== "undefined") {
    Object.defineProperty(navigator, "languages", {
      value: ["zh-CN"],
      configurable: true,
    });
    Object.defineProperty(navigator, "language", {
      value: "zh-CN",
      configurable: true,
    });
  }
  locale.setPreference("system");
  locale.refresh();
});
