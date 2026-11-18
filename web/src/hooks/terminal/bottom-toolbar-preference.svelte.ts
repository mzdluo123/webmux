import { resolveStorageKey, store, stored } from "../../platform/storage";
import { hasTouchInput } from "../../platform/touch-input";

export type BottomToolbarMode = "auto" | "shown" | "hidden";

const STORAGE_KEY = resolveStorageKey("bottomToolbar");

function readStoredChoice(): boolean | undefined {
  if (typeof window === "undefined") return undefined;
  const raw = stored("bottomToolbar");
  if (raw === "1") return true;
  if (raw === "0") return false;
  return undefined;
}

function readCoarsePointer(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    hasTouchInput()
  );
}

let choice = $state<boolean | undefined>(readStoredChoice());
let coarsePointer = $state<boolean>(readCoarsePointer());
let listenersAttached = false;

function attachListeners(): void {
  if (listenersAttached || typeof window === "undefined") return;
  listenersAttached = true;
  window.addEventListener("storage", (event) => {
    if (event.key === null || event.key === STORAGE_KEY) {
      choice = readStoredChoice();
    }
  });
  if (typeof window.matchMedia === "function") {
    const media = window.matchMedia("(any-pointer: coarse)");
    media.addEventListener("change", (event) => {
      coarsePointer = event.matches;
    });
  }
}

export const bottomToolbar = {
  get mode(): BottomToolbarMode {
    attachListeners();
    if (choice === undefined) return "auto";
    return choice ? "shown" : "hidden";
  },
  get shown(): boolean {
    attachListeners();
    return choice ?? coarsePointer;
  },
  set(next: boolean): void {
    attachListeners();
    choice = next;
    store("bottomToolbar", next ? "1" : "0");
  },
  reset(): void {
    attachListeners();
    choice = undefined;
    store("bottomToolbar", null);
  },
  refresh(): void {
    attachListeners();
    choice = readStoredChoice();
    coarsePointer = readCoarsePointer();
  },
};
