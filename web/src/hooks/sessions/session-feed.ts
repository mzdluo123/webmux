import { m } from "$lib/paraglide/messages.js";
import type { z } from "zod";
import { SessionEventSchema } from "../../contracts/schemas";
import { connect } from "../../platform/ws";
import type { Connection } from "../../platform/ws-contracts";

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
        throw new Error(m.s3_session_feed_must_be_json());
      const event = SessionEventSchema.parse(JSON.parse(data));
      observe?.(event);
      onEvent(event);
    },
    onState,
    onError,
  );
}
