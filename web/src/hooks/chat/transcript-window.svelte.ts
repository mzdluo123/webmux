import { untrack } from "svelte";
import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type { DisplayEntry } from "../../domains/chat/chat-model";

const VIRTUALIZE_THRESHOLD = 48;
const INITIAL_TAIL_PINNED = 8;
const TAIL_PINNED = 24;
const REVEAL_RADIUS = 8;
const OVERSCAN_MARGIN = "1200px 0px";
const MAX_HEIGHT_CACHE = 2000;

export function entryKey(entry: DisplayEntry): string {
  return `${entry.type}:${entry.id}`;
}

function estimateEntryHeight(entry: DisplayEntry): number {
  switch (entry.type) {
    case "message": {
      let newlines = 0;
      for (let i = 0; i < entry.text.length; i++) {
        if (entry.text.charCodeAt(i) === 10) newlines++;
      }
      const lines = Math.max(1, Math.ceil(entry.text.length / 68) + newlines);
      return Math.max(44, Math.min(480, 24 + lines * 22));
    }
    case "work":
      return 32;
    case "turnEnd":
      return entry.changes?.length || entry.error ? 40 : 24;
    default:
      return 36;
  }
}

/**
 * Keeps every transcript `<li>` anchor in the DOM while unmounting heavy
 * off-screen `<TimelineItem>` subtrees once a conversation grows large.
 * Previously mounted rows remember their measured height so scrolling back
 * preserves offsets without layout thrash.
 */
export function createTranscriptWindow(options: {
  entries: () => readonly DisplayEntry[];
  scroller: () => HTMLElement | null;
  documentScroll: () => boolean;
  following: () => boolean;
}) {
  const visibleKeys = new SvelteSet<string>();
  const measuredHeights = new SvelteMap<string, number>();
  const nodes = new SvelteMap<HTMLElement, string>();
  let pinnedIndex = $state<number | null>(null);
  let tailStart = $state(0);
  let hydrating = $state(false);
  let previousTotal = 0;
  let observer: IntersectionObserver | undefined;

  const active = $derived(options.entries().length > VIRTUALIZE_THRESHOLD);

  function rememberHeight(node: HTMLElement, key: string) {
    const height = node.getBoundingClientRect().height;
    if (height <= 0) return;
    if (measuredHeights.size >= MAX_HEIGHT_CACHE && !measuredHeights.has(key)) {
      const oldest = measuredHeights.keys().next().value;
      if (oldest !== undefined) measuredHeights.delete(oldest);
    }
    measuredHeights.set(key, height);
  }

  // Only advance the pinned tail window while following the bottom so new
  // streaming messages never unmount an entry above a reader in history.
  // When a large backlog arrives all at once (0 -> >48 entries), mount only
  // the bottom screenful on frame 1 and widen to the full tail buffer on the
  // next animation frame to avoid a single >100ms initial-mount LoAF.
  $effect(() => {
    const total = options.entries().length;
    const bulkInitial = previousTotal === 0 && total > VIRTUALIZE_THRESHOLD;
    previousTotal = total;
    if (bulkInitial && options.following()) {
      hydrating = true;
      tailStart = Math.max(0, total - INITIAL_TAIL_PINNED);
      const frame = requestAnimationFrame(() => {
        hydrating = false;
        const latest = options.entries().length;
        if (options.following()) {
          tailStart = Math.max(0, latest - TAIL_PINNED);
        }
      });
      return () => {
        cancelAnimationFrame(frame);
        hydrating = false;
      };
    }
    const desired = Math.max(0, total - TAIL_PINNED);
    if (options.following() || desired < tailStart) {
      tailStart = desired;
    }
  });

  $effect(() => {
    const enabled = active && !hydrating;
    const docScroll = options.documentScroll();
    const root = docScroll ? null : options.scroller();
    if (!enabled || (!docScroll && !root)) {
      observer?.disconnect();
      observer = undefined;
      visibleKeys.clear();
      return;
    }
    const next = new IntersectionObserver(
      (records) => {
        for (const record of records) {
          const node = record.target as HTMLElement;
          const key = nodes.get(node);
          if (!key) continue;
          if (record.isIntersecting) {
            visibleKeys.add(key);
          } else {
            if (node.contains(document.activeElement)) continue;
            rememberHeight(node, key);
            visibleKeys.delete(key);
          }
        }
      },
      { root, rootMargin: OVERSCAN_MARGIN },
    );
    observer = next;
    untrack(() => {
      for (const node of nodes.keys()) next.observe(node);
    });
    return () => {
      next.disconnect();
      if (observer === next) observer = undefined;
    };
  });

  function observeRow(node: HTMLElement, initialKey: string) {
    let key = initialKey;
    nodes.set(node, key);
    observer?.observe(node);
    return {
      update(nextKey: string) {
        if (nextKey === key) return;
        visibleKeys.delete(key);
        key = nextKey;
        nodes.set(node, key);
      },
      destroy() {
        observer?.unobserve(node);
        nodes.delete(node);
        visibleKeys.delete(key);
      },
    };
  }

  function reveal(messageId: string) {
    hydrating = false;
    const list = options.entries();
    const idx = list.findIndex(
      (entry) => entry.type === "message" && entry.id === messageId,
    );
    pinnedIndex = idx >= 0 ? idx : null;
  }

  function clearReveal() {
    pinnedIndex = null;
    tailStart = Math.max(0, options.entries().length - TAIL_PINNED);
  }

  function isMounted(
    entry: DisplayEntry,
    index: number,
    selectedMessage: string | null,
    targetMessage: string | null,
  ): boolean {
    const total = options.entries().length;
    if (total <= VIRTUALIZE_THRESHOLD) return true;
    if (index >= tailStart) return true;
    if (
      entry.type === "message" &&
      (entry.id === selectedMessage || entry.id === targetMessage)
    )
      return true;
    if (pinnedIndex !== null && Math.abs(index - pinnedIndex) <= REVEAL_RADIUS)
      return true;
    return visibleKeys.has(entryKey(entry));
  }

  function placeholderHeight(entry: DisplayEntry, key: string): number {
    return measuredHeights.get(key) ?? estimateEntryHeight(entry);
  }

  return {
    observeRow,
    reveal,
    clearReveal,
    isMounted,
    placeholderHeight,
  };
}
