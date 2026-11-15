import * as m from "$lib/paraglide/messages.js";
import { api, store, stored } from "./api";
import { decodeBase64Url } from "./base64url";
import {
  NoContentSchema,
  PushStatusSchema,
  PushSubscriptionRequestSchema,
  PushSubscriptionResultSchema,
  PushTestResultSchema,
  type PushReason,
  type PushStatus,
  type PushTestResult,
} from "../contracts/schemas";

// The server-side ID of this browser's subscription, under webmux.push.subscriptionId.
const SUBSCRIPTION_KEY = "push.subscriptionId";
const WORKER = "/sw.js";

export type PushSupport = {
  secure: boolean;
  serviceWorker: boolean;
  pushManager: boolean;
  notification: boolean;
  ios: boolean;
  standalone: boolean;
  permission: NotificationPermission | "unsupported";
};

export function detectSupport(): PushSupport {
  const nav = navigator as Navigator & { standalone?: boolean };
  const ios =
    /iPad|iPhone|iPod/.test(nav.userAgent) ||
    (nav.platform === "MacIntel" && nav.maxTouchPoints > 1);
  const notification = typeof Notification !== "undefined";
  return {
    secure: globalThis.isSecureContext === true,
    serviceWorker: "serviceWorker" in nav,
    pushManager: typeof PushManager !== "undefined",
    notification,
    ios,
    standalone:
      nav.standalone === true ||
      (typeof matchMedia === "function" &&
        matchMedia("(display-mode: standalone)").matches),
    permission: notification ? Notification.permission : "unsupported",
  };
}

// What the settings card shows. Insecure pages hide the push APIs entirely
// and iOS exposes them only to home screen apps, so both are diagnosed before
// missing capabilities.
export type PushAdvice =
  | "insecure"
  | "ios-home-screen"
  | "unsupported"
  | PushReason
  | "denied"
  | "ready";

export function advise(support: PushSupport, status?: PushStatus): PushAdvice {
  if (!support.secure) return "insecure";
  if (support.ios && !support.standalone) return "ios-home-screen";
  if (!support.serviceWorker || !support.pushManager || !support.notification)
    return "unsupported";
  if (status?.reason) return status.reason;
  if (support.permission === "denied") return "denied";
  return "ready";
}

function sameKey(current: ArrayBuffer | null, publicKey: string): boolean {
  if (!current) return false;
  const a = new Uint8Array(current);
  const b = decodeBase64Url(publicKey);
  return a.length === b.length && a.every((byte, i) => byte === b[i]);
}

export const pushStatus = () => api("/push", PushStatusSchema);

export const savedSubscriptionId = () => stored(SUBSCRIPTION_KEY);

// The browser subscription, if this page registered one before. It never
// registers the service worker.
export async function currentSubscription(): Promise<PushSubscription | null> {
  if (!("serviceWorker" in navigator)) return null;
  const registration = await navigator.serviceWorker.getRegistration("/");
  return (await registration?.pushManager.getSubscription()) ?? null;
}

async function register(
  subscription: PushSubscription,
  signal?: AbortSignal,
): Promise<string> {
  const { id } = await api(
    "/push/subscriptions",
    PushSubscriptionResultSchema,
    "POST",
    subscription.toJSON(),
    signal,
    PushSubscriptionRequestSchema,
  );
  signal?.throwIfAborted();
  store(SUBSCRIPTION_KEY, id);
  return id;
}

// requestPermission resolves once the user answers the browser's prompt. With
// Chrome's quiet permission UI that answer waits on a click on the address-bar
// bell, so callers should tell the user where to look while it is pending.
export async function requestPermission(): Promise<void> {
  const permission = await Notification.requestPermission();
  if (permission !== "granted")
    throw new Error(m.s1_push_permission_not_granted());
}

// enable must run inside the click that asked for it: browsers only show the
// permission prompt for a user gesture. A caller that already asked (to show
// where the prompt is) is not asked again.
export async function enable(publicKey: string): Promise<void> {
  if (Notification.permission !== "granted") await requestPermission();
  const registration = await navigator.serviceWorker.register(WORKER, {
    scope: "/",
  });
  await navigator.serviceWorker.ready;
  let subscription = await registration.pushManager.getSubscription();
  if (
    subscription &&
    !sameKey(subscription.options.applicationServerKey, publicKey)
  ) {
    await subscription.unsubscribe();
    subscription = null;
  }
  subscription ??= await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: decodeBase64Url(publicKey),
  });
  try {
    await register(subscription);
  } catch (error) {
    // A browser subscription the server refused would never be used.
    await subscription.unsubscribe().catch(() => undefined);
    store(SUBSCRIPTION_KEY, "");
    throw error;
  }
}

// disable always drops the browser subscription, even if the server cannot
// forget it; the server deletes it later when the push service reports 410.
export async function disable(): Promise<void> {
  const subscription = await currentSubscription();
  const id = savedSubscriptionId();
  store(SUBSCRIPTION_KEY, "");
  await subscription?.unsubscribe();
  if (id)
    await api(
      `/push/subscriptions/${encodeURIComponent(id)}`,
      NoContentSchema,
      "DELETE",
    );
}

export type SyncResult = "none" | "unavailable" | "synced" | "key-changed";

// sync re-registers an existing subscription idempotently, for example after
// the server rebuilt its store. Without one it does nothing: no worker
// registration and no permission prompt.
export async function sync(signal?: AbortSignal): Promise<SyncResult> {
  if (
    typeof Notification === "undefined" ||
    Notification.permission !== "granted"
  )
    return "none";
  const subscription = await currentSubscription();
  signal?.throwIfAborted();
  if (!subscription) return "none";
  const status = await api("/push", PushStatusSchema, "GET", undefined, signal);
  if (!status.available || !status.publicKey) return "unavailable";
  signal?.throwIfAborted();
  if (!sameKey(subscription.options.applicationServerKey, status.publicKey)) {
    await subscription.unsubscribe();
    store(SUBSCRIPTION_KEY, "");
    return "key-changed";
  }
  await register(subscription, signal);
  return "synced";
}

export type TestOutcome = PushTestResult & { ownRemoved: boolean };

// sendTest pushes a test notification to every registered device, so one
// device can check what the others show in the background. When the push
// service dropped this browser's own subscription, it is forgotten here too.
export async function sendTest(): Promise<TestOutcome> {
  const result = await api("/push/test", PushTestResultSchema, "POST");
  const id = savedSubscriptionId();
  const ownRemoved = !!id && result.removed.includes(id);
  if (ownRemoved) {
    store(SUBSCRIPTION_KEY, "");
    await (await currentSubscription())?.unsubscribe().catch(() => undefined);
  }
  return { ...result, ownRemoved };
}

// describeTest turns a test outcome into one line for the settings card.
export function describeTest(outcome: TestOutcome): {
  failed: boolean;
  message: string;
} {
  const { devices, accepted, removed, errors, ownRemoved } = outcome;
  if (devices === 0)
    return { failed: true, message: m.s1_push_test_no_devices() };
  if (accepted === devices)
    return {
      failed: false,
      message: m.s1_push_test_all_accepted({ devices: String(devices) }),
    };
  const sep = m.s1_list_sep_semicolon();
  const parts = [
    m.s1_push_test_partial_submitted({
      accepted: String(accepted),
      devices: String(devices),
    }),
  ];
  if (removed.length)
    parts.push(
      m.s1_push_test_expired_removed({ count: String(removed.length) }),
    );
  if (errors.length)
    parts.push(
      m.s1_push_test_send_failed({
        count: String(errors.length),
        errors: errors.join(sep),
      }),
    );
  if (ownRemoved) parts.push(m.s1_push_test_reenable_this_device());
  return { failed: errors.length > 0 || ownRemoved, message: parts.join(sep) };
}
