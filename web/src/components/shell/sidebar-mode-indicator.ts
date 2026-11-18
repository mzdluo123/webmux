import type { ShellMode } from "../../hooks/shared/shell.svelte";

export function sidebarModeIndicator(node: HTMLElement, initial: ShellMode) {
  const group = node.parentElement!;
  let mode = initial;

  function place(moving: boolean) {
    const button = group.querySelector<HTMLElement>(
      `[data-sidebar-mode="${mode}"]`,
    );
    if (!button) return;
    const bounds = group.getBoundingClientRect();
    const target = button.getBoundingClientRect();
    node.toggleAttribute("data-moving", moving && !node.hidden);
    node.style.setProperty("--mode-x", `${target.left - bounds.left}px`);
    node.style.setProperty("--mode-y", `${target.bottom - bounds.top - 2}px`);
    node.style.setProperty("--mode-width", `${target.width}px`);
    node.hidden = target.width === 0;
  }

  const resize = new ResizeObserver(() => place(false));
  resize.observe(group);
  for (const button of group.querySelectorAll("[data-sidebar-mode]")) {
    resize.observe(button);
  }
  place(false);
  return {
    update(next: ShellMode) {
      mode = next;
      place(true);
    },
    destroy() {
      resize.disconnect();
    },
  };
}
