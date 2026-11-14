import type { z } from "zod";
import { SessionEventSchema } from "../../contracts/schemas";
import { connect, type Connection } from "../../platform/ws";

export type SessionEvent = z.infer<typeof SessionEventSchema>;

export function connectSessions(
  onEvent: (event: SessionEvent) => void,
  onState: (state: Connection) => void,
  onError: (message: string) => void,
  observe?: (event: SessionEvent) => void,
) {
  return connect(
    "/ws/sessions",
    (data) => {
      if (typeof data !== "string")
        throw new Error("会话列表帧必须是 JSON 文本");
      const event = SessionEventSchema.parse(JSON.parse(data));
      observe?.(event);
      onEvent(event);
    },
    onState,
    onError,
  );
}
