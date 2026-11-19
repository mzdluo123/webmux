import { tick } from "svelte";
import { openSession } from "../shared/shell.svelte";

/** Allow the lazy session page and its first transcript update to mount. */
async function firstMessage(id: string) {
  const selector = `[data-user-message="${CSS.escape(id)}"]`;
  const find = () => document.querySelector<HTMLElement>(selector);
  if (!find()) {
    await new Promise<void>((resolve) => {
      const finish = () => {
        observer.disconnect();
        clearTimeout(timer);
        resolve();
      };
      const observer = new MutationObserver(() => {
        if (find()) finish();
      });
      const timer = setTimeout(finish, 350);
      observer.observe(document.body, { childList: true, subtree: true });
    });
  }
  return find();
}

export async function openStartedSession(
  id: string,
  input: HTMLTextAreaElement | null,
  chat: boolean,
) {
  if (
    !document.startViewTransition ||
    matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    openSession({ id });
    return;
  }
  const name = "quick-start-message";
  let message: HTMLElement | null = null;
  if (input && chat) input.style.viewTransitionName = name;
  const transition = document.startViewTransition(async () => {
    openSession({ id });
    await tick();
    if (chat) {
      message = await firstMessage(id);
      if (message) message.style.viewTransitionName = name;
    }
  });
  try {
    await transition.finished;
  } finally {
    if (input) input.style.removeProperty("view-transition-name");
    (message as HTMLElement | null)?.style.removeProperty(
      "view-transition-name",
    );
  }
}
