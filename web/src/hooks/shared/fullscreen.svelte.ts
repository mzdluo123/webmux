import { onMount } from "svelte";
import * as m from "$lib/paraglide/messages.js";
import {
  fullscreenOffered,
  fullscreenSupported,
  isStandalone,
} from "../../platform/capabilities";

export interface Fullscreen {
  // Whether the page offers a fullscreen control at all.
  readonly offered: boolean;
  readonly active: boolean;
  readonly busy: boolean;
  toggle(): Promise<void>;
}

// Document fullscreen for the terminal page. Fullscreen this page entered is
// left again when the page unmounts, including a request still in flight.
export function useFullscreen(setError: (message: string) => void): Fullscreen {
  let active = $state(false);
  let busy = $state(false);
  let owned = false;
  let mounted = false;
  // Neither capability changes while the page runs.
  const offered = fullscreenOffered({
    supported: fullscreenSupported(),
    standalone: isStandalone(),
  });

  onMount(() => {
    mounted = true;
    const changed = () => {
      active = Boolean(document.fullscreenElement);
      if (!active) owned = false;
    };
    changed();
    document.addEventListener("fullscreenchange", changed);
    return () => {
      mounted = false;
      document.removeEventListener("fullscreenchange", changed);
      if (owned && document.fullscreenElement === document.documentElement) {
        owned = false;
        void document.exitFullscreen().catch((err) => {
          console.error(m.s1_fullscreen_exit_on_leave_failed(), err);
        });
      }
    };
  });

  async function toggle() {
    if (busy) return;
    setError("");
    if (!document.fullscreenElement && !fullscreenSupported()) {
      setError(m.s1_fullscreen_unsupported());
      return;
    }
    busy = true;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        owned = true;
        // Fullscreen the document so body-level menus and dialogs remain visible.
        await document.documentElement.requestFullscreen({
          navigationUI: "hide",
        });
        if (
          !mounted &&
          owned &&
          document.fullscreenElement === document.documentElement
        ) {
          owned = false;
          await document.exitFullscreen();
        }
      }
    } catch (err) {
      if (!document.fullscreenElement) owned = false;
      if (mounted)
        setError(
          m.s1_fullscreen_toggle_failed_reason({
            error: err instanceof Error ? err.message : String(err),
          }),
        );
      else console.error(m.s1_fullscreen_toggle_failed(), err);
    } finally {
      if (mounted) busy = false;
    }
  }

  return {
    offered,
    get active() {
      return active;
    },
    get busy() {
      return busy;
    },
    toggle,
  };
}
