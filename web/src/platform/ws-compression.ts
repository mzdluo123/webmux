import { store, stored } from "./storage";
import { liveSockets } from "./ws-registry";

const compressionKey = "wsCompression";

// Per-device permessage-deflate preference. Browsers always offer the
// extension, so the server negotiates it only when the URL asks. Until the
// user chooses, devices whose primary pointer is coarse (phones, tablets)
// default to on: they are the ones on metered, slow links.
export function wsCompression(): boolean {
  const choice = stored(compressionKey);
  if (choice === "on") return true;
  if (choice === "off") return false;
  return (
    typeof matchMedia === "function" && matchMedia("(pointer: coarse)").matches
  );
}

export function setWsCompression(enabled: boolean) {
  store(compressionKey, enabled ? "on" : "off");
  for (const reopen of liveSockets) reopen();
}
