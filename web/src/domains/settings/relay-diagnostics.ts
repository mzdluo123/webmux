const reasons: Record<string, string> = {
  "authorization-revoked": "访问授权已撤销",
  "control-bootstrap-unavailable": "可信控制面引导配置不可用",
  "storage-unavailable": "受控存储不可用",
  "bulk-unavailable": "批量通道不可用；交互保持连接",
  "turn-unavailable": "TURN 暂不可用；交互保持 WSS",
  "p2p-plugin-unavailable": "未选择 P2P 插件；保持 WSS",
  "p2p-build-unavailable": "当前构建不含 P2P；保持 WSS",
  "stable-latency-advantage": "应用层采样连续满足稳定延迟收益",
};

// Status prose is not a channel for transport credentials or ICE material.
export function relayDiagnostic(value: string) {
  if (!value) return "";
  if (Object.hasOwn(reasons, value)) return reasons[value]!;
  if (
    value.length > 240 ||
    /[\r\n\0]|[A-Za-z0-9_+/=-]{32,}|https?:|wss?:|turns?:|\b\d{1,3}(?:\.\d{1,3}){3}\b|[a-f0-9]{0,4}:[a-f0-9:]+:|candidate:|fingerprint|sdp|cookie|authorization|private.key|certificate|credential|\b(?:token|secret|password|passwd|code|verifier|nonce|signature|key)\s*[:=]/i.test(
      value,
    )
  ) {
    return "诊断包含敏感传输材料，已隐藏。请检查宿主的脱敏状态。";
  }
  return value;
}

export function relayEndpoint(value: string) {
  if (!value) return "未分配";
  try {
    const url = new URL(value);
    if (
      !["https:", "wss:"].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      url.hostname.includes(":") ||
      /^\d+(\.\d+){3}$/.test(url.hostname)
    )
      return "不可显示的端点";
    return url.origin + (url.pathname === "/" ? "" : url.pathname);
  } catch {
    return "不可显示的端点";
  }
}
