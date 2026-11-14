import { z } from "zod";

export const XR_PANEL_LIMIT = 16;
const coordinate = z.number().finite().min(-8).max(8);
const xrPanelMetadataSchema = z.object({
  id: z.string().min(1).max(256),
  sessionId: z.string().min(1).max(128),
  kind: z.enum(["terminal", "chat", "code", "diff"]),
  sourceId: z.string().min(1).max(256),
  position: z.tuple([
    coordinate,
    coordinate,
    z.number().finite().min(-12).max(-0.5),
  ]),
  size: z.tuple([
    z.number().finite().min(0.4).max(4),
    z.number().finite().min(0.3).max(3),
  ]),
  visible: z.boolean(),
});
const xrPanelSchema = xrPanelMetadataSchema.refine(
  (panel) => panel.kind !== "terminal" || panel.sourceId === "terminal",
);
export type XrPanelLayout = z.infer<typeof xrPanelSchema>;
const xrLayoutSchema = z.object({
  version: z.union([z.literal(1), z.literal(2)]),
  panels: z.array(z.unknown()).max(XR_PANEL_LIMIT),
});

/** Invalid individual records do not discard their valid neighbours. */
export function decodeXrLayout(
  value: unknown,
  sessions?: ReadonlySet<string>,
): XrPanelLayout[] | undefined {
  const document = xrLayoutSchema.safeParse(value);
  if (!document.success) return undefined;
  const seen = new Set<string>();
  let readable = document.data.panels.length === 0;
  const panels = document.data.panels.flatMap((record) => {
    let candidate = record;
    if (document.data.version === 1) {
      const legacy = xrPanelMetadataSchema
        .omit({ sourceId: true })
        .safeParse(record);
      if (!legacy.success) return [];
      const { id, sessionId, kind } = legacy.data;
      const prefix = `${kind}:${sessionId}:`;
      const sourceId =
        kind === "terminal"
          ? id === `terminal:${sessionId}`
            ? "terminal"
            : ""
          : id.startsWith(prefix)
            ? id.slice(prefix.length)
            : "";
      candidate = { ...legacy.data, sourceId };
    }
    const panel = xrPanelSchema.safeParse(candidate);
    if (!panel.success) return [];
    readable = true;
    if (
      (sessions && !sessions.has(panel.data.sessionId)) ||
      seen.has(panel.data.id)
    )
      return [];
    seen.add(panel.data.id);
    return [panel.data];
  });
  return readable ? panels : undefined;
}

/** Serialization deliberately projects only display metadata, never source content. */
export function encodeXrLayout(panels: readonly XrPanelLayout[]) {
  return {
    version: 2 as const,
    panels: panels.slice(0, XR_PANEL_LIMIT).flatMap((panel) => {
      const parsed = xrPanelSchema.safeParse(panel);
      return parsed.success ? [parsed.data] : [];
    }),
  };
}

export function defaultXrPanel(
  id: string,
  sessionId: string,
  kind: XrPanelLayout["kind"],
  index: number,
  sourceId = kind === "terminal"
    ? "terminal"
    : id.slice(`${kind}:${sessionId}:`.length),
): XrPanelLayout {
  return {
    id,
    sessionId,
    kind,
    sourceId,
    position: [((index % 3) - 1) * 1.65, -Math.floor(index / 3) * 1.25, -2.4],
    size: [1.6, 1.1],
    visible: true,
  };
}

export function boundXrPanel(panel: XrPanelLayout): void {
  panel.position[0] = Math.max(-8, Math.min(8, panel.position[0]));
  panel.position[1] = Math.max(-8, Math.min(8, panel.position[1]));
  panel.position[2] = Math.max(-12, Math.min(-0.5, panel.position[2]));
  panel.size[0] = Math.max(0.4, Math.min(4, panel.size[0]));
  panel.size[1] = Math.max(0.3, Math.min(3, panel.size[1]));
}
