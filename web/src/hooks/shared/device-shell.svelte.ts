import { navigate } from "../../platform/api";
import { headsetMode } from "./headset-mode.svelte";
import { shellUi } from "./shell.svelte";
import { useTvNavigation } from "./tv-navigation.svelte";

/** Root device modes share the shell's drawer and route Back boundary. */
export function useDeviceShell() {
  useTvNavigation(() => {
    if (shellUi.drawerOpen) {
      shellUi.drawerOpen = false;
      return true;
    }
    const routeBack = new CustomEvent("webmux:tv-back", {
      cancelable: true,
      detail: "route",
    });
    window.dispatchEvent(routeBack);
    if (routeBack.defaultPrevented) return true;
    if (location.pathname === "/") return false;
    navigate("/");
    return true;
  });
  $effect(() => {
    document.documentElement.dataset.headsetMode = String(headsetMode.enabled);
    return () => {
      delete document.documentElement.dataset.headsetMode;
    };
  });
}
