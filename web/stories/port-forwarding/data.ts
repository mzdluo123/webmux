// Sample data for the port-forwarding prototype (openspec change
// `port-forwarding`). Shapes mirror the planned `Forward` and
// `ListeningPort` contract types; nothing here talks to a server.

export type PrefixMode = "keep" | "strip";

export interface ForwardOptions {
  rewriteHost: boolean;
  rewriteOrigin: boolean;
  forwardedHeaders: boolean;
  rewriteResponse: boolean;
  prefix: PrefixMode;
}

export interface Forward {
  port: number;
  label: string;
  session: string;
  process: string;
  upstream: "listening" | "unreachable";
  options: ForwardOptions;
}

export interface DetectedPort {
  port: number;
  process: string;
  session: string;
}

export const WEBMUX_PORT = 7681;

export const defaultOptions: ForwardOptions = {
  rewriteHost: true,
  rewriteOrigin: true,
  forwardedHeaders: true,
  rewriteResponse: true,
  prefix: "keep",
};

export const sampleForwards: Forward[] = [
  {
    port: 5173,
    label: "",
    session: "web",
    process: "node",
    upstream: "listening",
    options: { ...defaultOptions },
  },
  {
    port: 8000,
    label: "docs 静态预览",
    session: "",
    process: "",
    upstream: "unreachable",
    options: { ...defaultOptions, prefix: "strip" },
  },
];

export const sampleDetected: DetectedPort[] = [
  { port: 6006, process: "node", session: "storybook" },
  { port: 9229, process: "node", session: "api-server" },
];

export function forwardPath(port: number): string {
  return `/forward/${port}/`;
}

/** The inline error the add form shows, or "" when the port is valid. */
export function portError(raw: string, taken: number[]): string {
  if (!raw.trim()) return "";
  if (!/^[1-9]\d{0,4}$/.test(raw.trim())) return "请输入 1–65535 的端口号";
  const port = Number(raw);
  if (port > 65535) return "请输入 1–65535 的端口号";
  if (port === WEBMUX_PORT) return "这是 WebMux 自身的端口，不能转发";
  if (taken.includes(port)) return "该端口已转发";
  return "";
}
