import { m } from "$lib/paraglide/messages.js";
import type { CodeDiff, CodeDocument } from "../../contracts/schemas";
import { APIError } from "../../platform/api";

export const isImageType = (mediaType: string | undefined) =>
  !!mediaType?.startsWith("image/");

export function canPreviewDocumentImage(
  doc: CodeDocument | undefined,
  shownDiff: CodeDiff | undefined,
  svgFile: boolean,
  svgView: "preview" | "source",
) {
  return (
    !!doc &&
    doc.source !== "empty" &&
    (shownDiff
      ? shownDiff.kind === "image" &&
        (doc.kind === "image" ||
          (doc.kind === "text" && doc.mediaType === "image/svg+xml"))
      : doc.kind === "image" || (svgFile && svgView === "preview"))
  );
}

export function isDocumentImage(
  doc: CodeDocument | undefined,
  shownDiff: CodeDiff | undefined,
) {
  return (
    !!doc &&
    (shownDiff ? shownDiff.kind === "image" : isImageType(doc.mediaType))
  );
}

// Raw snapshot bytes. File snapshots ignore `side`; Diff snapshots require it.
export function codeRawPath(
  id: string,
  snapshotId: string,
  side?: "old" | "new",
  download = false,
) {
  const query = new URLSearchParams();
  if (side) query.set("side", side);
  if (download) query.set("download", "1");
  const search = query.toString();
  return `/sessions/${id}/code/snapshots/${snapshotId}/raw${search ? `?${search}` : ""}`;
}

const formats: Record<string, string> = {
  "image/png": "PNG",
  "image/jpeg": "JPEG",
  "image/gif": "GIF",
  "image/webp": "WebP",
  "image/avif": "AVIF",
  "image/bmp": "BMP",
  "image/x-icon": "ICO",
  "image/svg+xml": "SVG",
};
export const imageFormat = (mediaType: string) =>
  formats[mediaType] ?? mediaType;

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KiB", "MiB", "GiB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1).replace(/\.0$/, "")} ${units[unit]}`;
}

export const basename = (path: string) =>
  path.slice(path.lastIndexOf("/") + 1) || "download";

// Explains a failed raw-bytes request. Fetch rejects with TypeError when the
// connection fails, including a response the server aborted mid-transfer.
export function rawError(
  error: unknown,
  action: "preview" | "download" | (string & {}),
) {
  const label =
    action === "preview"
      ? m.s5_action_preview()
      : action === "download"
        ? m.s5_action_download()
        : action;
  if (error instanceof APIError)
    switch (error.status) {
      case 401:
      case 403:
        return m.s5_raw_err_auth({ action: label });
      case 404:
        return m.s5_raw_err_missing({ action: label });
      case 409:
        return m.s5_raw_err_changed({ action: label });
      case 410:
        return m.s5_raw_err_expired({ action: label });
      case 413:
        return m.s5_raw_err_too_large({ action: label });
      case 429:
        return m.s5_raw_err_too_many({ action: label });
      default:
        return m.s5_raw_err_generic({ action: label, message: error.message });
    }
  if (error instanceof TypeError)
    return m.s5_raw_err_network({ action: label });
  return m.s5_raw_err_generic({
    action: label,
    message: error instanceof Error ? error.message : String(error),
  });
}
