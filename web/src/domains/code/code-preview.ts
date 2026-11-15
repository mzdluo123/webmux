import * as m from "$lib/paraglide/messages.js";

export const pdfMediaType = "application/pdf";

// Text formats with a read-only preview beside the source view.
export type PreviewFormat = "markdown" | "html" | "csv" | "tsv" | "json";

const formats: [RegExp, PreviewFormat][] = [
  [/\.(?:md|markdown|mdown|mkd)$/i, "markdown"],
  [/\.html?$/i, "html"],
  [/\.csv$/i, "csv"],
  [/\.tsv$/i, "tsv"],
  [/\.json$/i, "json"],
];

export const previewFormat = (path: string): PreviewFormat | undefined =>
  formats.find(([pattern]) => pattern.test(path))?.[1];

// Names the preview in the narrow code menu.
export function previewFormatLabel(format: PreviewFormat): string {
  switch (format) {
    case "markdown":
      return m.s5_preview_markdown();
    case "html":
      return m.s5_preview_html();
    case "csv":
    case "tsv":
      return m.s5_preview_table();
    case "json":
      return m.s5_preview_json();
  }
}
