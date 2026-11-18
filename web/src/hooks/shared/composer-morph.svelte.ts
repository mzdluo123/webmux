import { useHeightMorph } from "./height-morph.svelte";

export function useComposerMorph(
  getCard: () => HTMLElement | null,
  isCompact: () => boolean,
) {
  useHeightMorph(getCard, isCompact, "[data-composer-action]");
}
