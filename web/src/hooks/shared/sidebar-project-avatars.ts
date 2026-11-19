import { createContext, onDestroy } from "svelte";
import { SvelteMap } from "svelte/reactivity";
import { repositoryRemotes } from "../../platform/api";

const [getSidebarProjectAvatars, setAvatars] = createContext<{
  src(repositoryId: string): string | null | undefined;
  load(repositoryId: string): void;
}>();

export { getSidebarProjectAvatars };

export function provideSidebarProjectAvatars() {
  const repositories = new SvelteMap<string, string | null>();
  const owners = new Map<string, Promise<string | null>>();
  const urls = new Set<string>();
  const controller = new AbortController();
  const { signal } = controller;

  async function loadAvatar(owner: string): Promise<string | null> {
    const response = await fetch(
      `https://avatars.githubusercontent.com/${owner}?s=48`,
      { credentials: "omit", referrerPolicy: "no-referrer", signal },
    );
    if (!response.ok) return null;
    const blob = await response.blob();
    if (signal.aborted) return null;
    // Keep one decoded source for both layouts and the mobile drawer. They
    // can be replaced without consulting the GitHub image cache again.
    const url = URL.createObjectURL(blob);
    urls.add(url);
    try {
      const image = new Image();
      image.src = url;
      await image.decode();
      return url;
    } catch {
      urls.delete(url);
      URL.revokeObjectURL(url);
      return null;
    }
  }

  async function resolve(repositoryId: string) {
    const { remotes } = await repositoryRemotes(repositoryId, signal);
    if (signal.aborted) return null;
    const origin = remotes.find((remote) => remote.name === "origin");
    // Only the public GitHub account is sent to the avatar host, never the
    // repository path or any user info in the remote.
    const owner = origin?.fetchUrl
      .match(
        /^(?:https?:\/\/(?:[^/@]+@)?github\.com\/|ssh:\/\/git@github\.com(?::22)?\/|git@github\.com:)([a-z\d][a-z\d-]{0,38})\/[\w.-]+\/?$/i,
      )?.[1]
      ?.toLowerCase();
    if (!owner) return null;
    let avatar = owners.get(owner);
    if (!avatar) {
      avatar = loadAvatar(owner).catch(() => null);
      owners.set(owner, avatar);
    }
    return avatar;
  }

  setAvatars({
    src: (repositoryId) => repositories.get(repositoryId),
    load(repositoryId) {
      if (repositories.has(repositoryId)) return;
      repositories.set(repositoryId, null);
      void resolve(repositoryId)
        .catch(() => null)
        .then((src) => {
          if (src && !signal.aborted) repositories.set(repositoryId, src);
        });
    },
  });
  onDestroy(() => {
    controller.abort();
    for (const url of urls) URL.revokeObjectURL(url);
  });
}
