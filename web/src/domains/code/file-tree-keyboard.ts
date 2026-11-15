export function navigateFileTreeKey(event: KeyboardEvent) {
  const target = (event.target as HTMLElement).closest<HTMLButtonElement>(
    "button[data-file-entry]",
  );
  if (!target) return;
  const rows = [
    ...(event.currentTarget as HTMLElement)
      .closest("[data-file-tree]")!
      .querySelectorAll<HTMLButtonElement>("button[data-file-entry]"),
  ].filter((row) => !row.disabled && row.getClientRects().length > 0);
  const index = rows.indexOf(target);
  let next: HTMLButtonElement | undefined;
  if (event.key === "ArrowDown")
    next = rows[Math.min(index + 1, rows.length - 1)];
  else if (event.key === "ArrowUp") next = rows[Math.max(index - 1, 0)];
  else if (event.key === "Home") next = rows[0];
  else if (event.key === "End") next = rows.at(-1);
  else if (event.key === "ArrowRight") {
    if (target.getAttribute("aria-expanded") === "false") target.click();
    else if (target.getAttribute("aria-expanded") === "true")
      next = rows[index + 1];
  } else if (event.key === "ArrowLeft") {
    if (target.getAttribute("aria-expanded") === "true") target.click();
    else
      next =
        target
          .closest("li")
          ?.parentElement?.closest("li")
          ?.querySelector<HTMLButtonElement>("button[data-file-entry]") ??
        rows[0];
  } else return;
  event.preventDefault();
  next?.focus();
}
