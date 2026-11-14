import { z } from "zod";

export const XR_PANEL_LIMIT = 16;
const coordinate = z.number().finite().min(-8).max(8);
const xrPanelSchema = z.object({
  id: z.string().min(1).max(256),
  sessionId: z.string().min(1).max(128),
  kind: z.enum(["terminal", "chat", "code", "diff"]),
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
export type XrPanelLayout = z.infer<typeof xrPanelSchema>;
const xrLayoutSchema = z.object({
  version: z.literal(1),
  panels: z.array(z.unknown()).max(XR_PANEL_LIMIT),
});

/** Invalid individual records do not discard their valid neighbours. */
export function decodeXrLayout(
  value: unknown,
  sessions: ReadonlySet<string>,
): XrPanelLayout[] {
  const document = xrLayoutSchema.safeParse(value);
  if (!document.success) return [];
  const seen = new Set<string>();
  return document.data.panels.flatMap((record) => {
    const panel = xrPanelSchema.safeParse(record);
    if (
      !panel.success ||
      !sessions.has(panel.data.sessionId) ||
      seen.has(panel.data.id)
    )
      return [];
    seen.add(panel.data.id);
    return [panel.data];
  });
}

/** Serialization deliberately projects only display metadata, never source content. */
export function encodeXrLayout(panels: readonly XrPanelLayout[]) {
  return {
    version: 1 as const,
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
): XrPanelLayout {
  return {
    id,
    sessionId,
    kind,
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
