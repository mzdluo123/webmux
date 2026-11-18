import { matchRoute } from "./route";

// Keep this allowlist aligned with the host's authReturnTo contract.
const gitQueryKeys: Record<string, true> = {
  view: true,
  scope: true,
  branch: true,
  commit: true,
  q: true,
  author: true,
  since: true,
  until: true,
  path: true,
  returnSession: true,
};

function hasControlCharacters(value: string) {
  for (let index = 0; index < value.length; index++) {
    const code = value.charCodeAt(index);
    if (code < 32 || code === 127) return true;
  }
  return false;
}

/** Only host-approved app routes and Git queries; never auth endpoints or fragments. */
export function relayLoginHref(relative: string) {
  let returnTo = "/";
  if (
    relative.startsWith("/") &&
    !relative.startsWith("//") &&
    !relative.includes("\\") &&
    !hasControlCharacters(relative) &&
    relative.length <= 2048
  ) {
    const url = new URL(relative, "https://webmux.invalid");
    const route = matchRoute(url.pathname);
    if (
      (url.pathname === "/" ||
        url.pathname === "/app" ||
        (route.name !== "sessions" && route.name !== "forward-auth")) &&
      url.pathname === relative.split(/[?#]/, 1)[0]
    ) {
      returnTo = url.pathname;
      if (route.name === "git") {
        const query = new URLSearchParams();
        for (const [key, value] of url.searchParams) {
          if (
            Object.hasOwn(gitQueryKeys, key) &&
            url.searchParams.getAll(key).length === 1 &&
            !hasControlCharacters(value)
          )
            query.set(key, value);
        }
        const search = query.toString();
        if (search) returnTo += `?${search}`;
      }
    }
  }
  return `/auth/relay/start?${new URLSearchParams({ returnTo })}`;
}
