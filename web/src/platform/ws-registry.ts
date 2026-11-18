import type { ManagedStreamStatus } from "./ws-contracts";

// Every live connection, so a compression change can reattach them at once.
export const liveSockets = new Set<() => void>();
export const streamStatuses = new Map<number, ManagedStreamStatus>();
const streamObservers = new Set<(streams: ManagedStreamStatus[]) => void>();
let streamCounter = 0;

export function nextManagedStreamID() {
  return ++streamCounter;
}

export function publishStreams() {
  for (const observer of streamObservers)
    observer([...streamStatuses.values()]);
}

export function observeManagedStreams(
  observer: (streams: ManagedStreamStatus[]) => void,
) {
  streamObservers.add(observer);
  observer([...streamStatuses.values()]);
  return () => {
    streamObservers.delete(observer);
  };
}
