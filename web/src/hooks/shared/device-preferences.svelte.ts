import type { z } from "zod";
import { store, stored } from "../../platform/api";

/** Keep changes reactive even when this browser cannot write to storage. */
export function deviceNumberPreference(key: string, schema: z.ZodType<number>) {
  const saved = stored(key);
  let value = $state(saved ? schema.safeParse(saved).data : undefined);
  return {
    get value() {
      return value;
    },
    set(next: number) {
      value = next;
      store(key, String(next));
    },
    reset(this: void) {
      value = undefined;
      store(key, null);
    },
  };
}

/** The device default applies to missing and unrecognized stored values. */
export function deviceToggle(key: string, enabledByDefault: boolean) {
  const saved = stored(key);
  let enabled = $state(enabledByDefault ? saved !== "0" : saved === "1");
  return {
    get enabled() {
      return enabled;
    },
    set enabled(next: boolean) {
      enabled = next;
      store(key, next ? "1" : "0");
    },
  };
}

/** Versioned device data stays usable when storage is absent or malformed. */
export function deviceObjectPreference<T>(
  key: string,
  schema: z.ZodType<T>,
  fallback: () => T,
) {
  function readValue(): T {
    const saved = stored(key);
    if (!saved) return fallback();
    try {
      const parsed = schema.safeParse(JSON.parse(saved));
      if (parsed.success) return parsed.data;
    } catch {
      // A truncated JSON document is not a reason to block the workspace.
    }
    store(key, null);
    return fallback();
  }
  let value = $state.raw<T>(readValue());
  return {
    get value() {
      return value;
    },
    set(next: T) {
      const parsed = schema.safeParse(next);
      if (!parsed.success) return false;
      value = parsed.data;
      store(key, JSON.stringify(parsed.data));
      return true;
    },
    reset() {
      value = fallback();
      store(key, null);
    },
  };
}
