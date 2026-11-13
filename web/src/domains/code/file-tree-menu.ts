export function openFileTreeMenu<T>(
  event: MouseEvent,
  node: T,
  menu?: (
    node: T,
    at: HTMLElement | { x: number; y: number },
    row: HTMLElement,
  ) => void,
) {
  if (!menu) return;
  event.preventDefault();
  // Touch menus come from long press; keyboard context menus anchor to the row.
  if (event instanceof PointerEvent && event.pointerType === "touch") return;
  const row = event.currentTarget as HTMLElement;
  const box = row.getBoundingClientRect();
  const inside =
    event.clientX >= box.left &&
    event.clientX <= box.right &&
    event.clientY >= box.top &&
    event.clientY <= box.bottom;
  menu(node, inside ? { x: event.clientX, y: event.clientY } : row, row);
}
