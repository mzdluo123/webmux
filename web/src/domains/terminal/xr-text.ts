const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

/** Spatial clipping and scrolling never split a visible grapheme. */
export function xrGraphemes(text: string) {
  return Array.from(segmenter.segment(text), (part) => part.segment);
}
