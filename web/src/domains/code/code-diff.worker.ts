import { locale, type SupportedLocale } from "$lib/locale";
import { computeDiff } from "./code-diff";

self.onmessage = (
  event: MessageEvent<{
    snapshotId: string;
    old: string;
    new: string;
    locale?: SupportedLocale;
  }>,
) => {
  if (event.data.locale) locale.setPreference(event.data.locale);
  const { snapshotId, old, new: next } = event.data;
  self.postMessage({ snapshotId, result: computeDiff(old, next) });
};
