/** A filename suggestion only; the server validates transport and target. */
export function suggestProjectClonePath(
  parent: string,
  remote: string,
): string {
  const address = remote.trim().replace(/\/+$/, "");
  if (!parent || /[?#\r\n]/.test(address)) return "";
  const name = address
    .slice(Math.max(address.lastIndexOf("/"), address.lastIndexOf(":")) + 1)
    .replace(/\.git$/i, "");
  if (!/^[a-zA-Z0-9_.-]+$/.test(name) || name === "." || name === "..")
    return "";
  return `${parent.replace(/\/+$/, "")}/${name}`;
}
