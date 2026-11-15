<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import type { RelaySettingsView } from "../../hooks/settings/relay-settings.svelte";
  import {
    relayDiagnostic,
    relayEndpoint,
  } from "../../domains/settings/relay-diagnostics";

  let {
    view,
    onrefresh,
    onpair,
    oncancel,
    onenable,
    ondisconnect,
    onreconnect,
    ondelete,
  }: {
    view: RelaySettingsView;
    onrefresh: () => void;
    onpair: (rebind: boolean) => void;
    oncancel: () => void;
    onenable: () => void;
    ondisconnect: () => void;
    onreconnect: () => void;
    ondelete: () => void;
  } = $props();
  let confirmation = $state<
    "pair" | "rebind" | "enable" | "disconnect" | "reconnect" | "delete" | null
  >(null);
  const status = $derived(view.status);
  const blocked = $derived(view.busy || !!view.enrollment);
  const stateLabel = {
    off: "已关闭",
    connecting: "连接中",
    ready: "已就绪",
    degraded: "批量通道降级",
    failed: "连接失败",
  };
  const reachability = {
    unknown: "尚未确认",
    reachable: "可达",
    unavailable: "不可用",
    unassigned: "未分配",
  };
  const confirmTitle = $derived(
    confirmation === "pair"
      ? "配对 GitHub 主机？"
      : confirmation === "rebind"
        ? "重新绑定主机？"
        : confirmation === "enable"
          ? "启用公网 Shell 访问？"
          : confirmation === "delete"
            ? "删除 Relay 绑定？"
            : confirmation === "reconnect"
              ? "重新连接 Relay？"
              : "断开 Relay？",
  );

  function confirm() {
    const action = confirmation;
    confirmation = null;
    if (action === "pair" || action === "rebind") onpair(action === "rebind");
    else if (action === "enable") onenable();
    else if (action === "disconnect") ondisconnect();
    else if (action === "reconnect") onreconnect();
    else if (action === "delete") ondelete();
  }
</script>

<Card.Root flat>
  <Card.Header>
    <Card.Title>主机配对与访问</Card.Title>
    <Card.Description
      >主机配对与公网访问独立控制。默认关闭，不影响本机及其他公网入口。</Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-5">
    {#if view.loading}
      <p role="status" aria-busy="true">正在读取 Relay 设置…</p>
    {:else if !status}
      <InlineNotice tone="warning">当前宿主未提供可用 Relay 状态。</InlineNotice
      >
    {:else}
      <div class="flex items-center justify-between gap-4">
        <Label for="relay-enabled">允许 Relay 公网访问</Label>
        <Switch
          id="relay-enabled"
          checked={status.enabled}
          disabled={blocked ||
            (!status.enabled &&
              (!status.paired ||
                !status.tokenStrong ||
                !status.storageAvailable))}
          onCheckedChange={(enabled) => {
            confirmation = enabled ? "enable" : "disconnect";
          }}
        />
      </div>
      <p role="status" class="text-sm">
        {stateLabel[status.state]} · {status.paired
          ? "已配对"
          : status.pairing
            ? "配对中"
            : "未配对"} · WSS 基线 {status.readyConnections}/2
      </p>
      {#if !status.hybridSupported}
        <InlineNotice tone="warning"
          >{status.hybridUnavailableReason === "p2p-build-unavailable"
            ? "当前构建不包含 P2P 实现"
            : "当前未选择 p2p 插件"}；配对与 WSS 基础访问仍可使用，直连/TURN
          交互优化不可用。不会加载禁用实现或发起 P2P 请求。本机 P2P
          开关不影响已授权所有者的自动优化。</InlineNotice
        >
      {/if}
      {#if !status.controlEndpoint}
        <InlineNotice tone="warning"
          >可信控制面尚未配置，当前不可配对或启用。由宿主部署配置端点与初始信任；此页面不接受任意节点或
          IP。</InlineNotice
        >
      {/if}
      {#if !status.tokenStrong}
        <InlineNotice tone="warning"
          >启用要求宿主恢复 Token 至少 43 个字符。请在宿主配置中更换；不要把
          Token 交给控制面。</InlineNotice
        >
      {/if}
      {#if !status.storageAvailable}
        <InlineNotice tone="danger"
          >受控存储不可用；禁止新建公网连接。关闭操作仍会立即停止访问。</InlineNotice
        >
      {/if}
      {#if status.ownerId}
        <p class="text-sm wrap-anywhere">
          GitHub 所有者：{status.ownerLogin || "账号"} · {status.ownerId}
        </p>
      {/if}
      {#if status.boundPublicUrl}
        <div class="space-y-2">
          <p class="text-sm text-muted-foreground">
            固定公共 HTTPS 地址（不含 Token；地址存在不代表入口已授权）
          </p>
          <p class="text-sm wrap-anywhere">
            {relayEndpoint(status.boundPublicUrl)}
          </p>
          {#if status.controlReady && status.publicUrl}
            <Button
              href={new URL("/auth/relay/start", status.publicUrl).href}
              variant="outline">通过所有者账号打开 WebMux</Button
            >
          {/if}
        </div>
      {/if}
      {#if status.state === "degraded" || (status.controlReady && !status.bulkReady)}
        <InlineNotice tone="warning"
          >交互仍可用；上传、下载与转发的新请求不可用，不会挤入交互通道。</InlineNotice
        >
      {/if}
      {#if status.error || status.readinessError}
        <InlineNotice tone="danger"
          >{relayDiagnostic(
            status.error || status.readinessError,
          )}</InlineNotice
        >
      {/if}
      {#if status.reconnectReason}
        <InlineNotice tone="warning"
          >{relayDiagnostic(status.reconnectReason)}</InlineNotice
        >
      {/if}
      {#if view.enrollment}
        <section class="space-y-3" aria-label="GitHub 设备授权">
          <p class="text-sm">在 GitHub 输入一次性配对码：</p>
          <p class="font-mono text-2xl" aria-label="GitHub 配对码">
            {view.enrollment.userCode}
          </p>
          <p class="text-xs text-muted-foreground">
            有效期至 {new Date(
              view.enrollment.expiresAt * 1000,
            ).toLocaleTimeString()}。等待授权，按 GitHub 要求间隔轮询。
          </p>
          <div class="flex flex-wrap gap-3">
            <Button
              href={view.enrollment.verificationUrl}
              target="_blank"
              rel="noopener noreferrer">前往 GitHub 验证</Button
            >
            <Button variant="outline" onclick={oncancel} disabled={view.busy}
              >取消配对</Button
            >
          </div>
        </section>
      {:else}
        <div class="flex flex-wrap gap-3">
          <Button
            disabled={blocked ||
              !status.storageAvailable ||
              !status.controlEndpoint}
            onclick={() => {
              confirmation = status.paired ? "rebind" : "pair";
            }}
          >
            {status.paired ? "重新绑定 GitHub" : "开始 GitHub 配对"}
          </Button>
          {#if status.paired}
            <Button
              variant="outline"
              disabled={blocked || !status.enabled}
              onclick={() => {
                confirmation = "reconnect";
              }}>重新连接</Button
            >
            <Button
              variant="destructive-outline"
              disabled={blocked}
              onclick={() => {
                confirmation = "delete";
              }}>删除绑定</Button
            >
          {/if}
        </div>
      {/if}
      {#if status.sessions.length}
        <section class="space-y-2" aria-label="宿主观测的候选连接">
          <h3 class="text-sm font-medium">宿主观测的候选连接</h3>
          {#each status.sessions as session (session.id)}
            <p class="text-sm wrap-anywhere">
              会话 {session.id.slice(0, 8)}… · {session.path} · {relayDiagnostic(
                session.reason,
              ) || "无额外原因"}
            </p>
          {/each}
          <p class="text-xs text-muted-foreground">
            这里只表示已选 ICE
            对的连接类型，不证明业务已切换。实际选择以当前浏览器的逐流状态为准。
          </p>
        </section>
      {/if}
      <details class="space-y-3">
        <summary class="cursor-pointer text-sm">有效配置与脱敏诊断</summary>
        <dl class="grid min-w-0 grid-cols-1 gap-2 text-sm sm:grid-cols-2">
          <dt class="text-muted-foreground">配置的控制面</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.controlEndpoint)} · {status.controlSource}
          </dd>
          <dt class="text-muted-foreground">控制面引导信任</dt>
          <dd>
            {status.controlTrustPins.length} 个已配置 SPKI pin；不会从 URL 或证书自动学习
          </dd>
          <dt class="text-muted-foreground">控制面可达性</dt>
          <dd>{reachability[status.controlReachability]}</dd>
          <dt class="text-muted-foreground">分配来源</dt>
          <dd>
            {status.assignmentSource === "authenticated-control"
              ? "已认证控制面"
              : "未分配"} · 版本 {status.assignmentRevision}
          </dd>
          <dt class="text-muted-foreground">连接器 / 数据来源</dt>
          <dd class="wrap-anywhere">
            {relayEndpoint(status.connectorUrl)} / {relayEndpoint(
              status.dataOrigin,
            )}
          </dd>
          <dt class="text-muted-foreground">数据面可达性</dt>
          <dd>{reachability[status.dataReachability]}</dd>
          <dt class="text-muted-foreground">节点 / 能力</dt>
          <dd class="wrap-anywhere">
            {status.nodeId || "未分配"} · {status.capabilities.join(" / ") ||
              "无"}
          </dd>
          <dt class="text-muted-foreground">分配配置 / 实际应用版本</dt>
          <dd>
            {status.configRevision} / {status.appliedRevision === "0"
              ? "未知"
              : status.appliedRevision}
          </dd>
          <dt class="text-muted-foreground">TCP 算法</dt>
          <dd>{relayDiagnostic(status.tcpAlgorithm) || "unknown"}</dd>
        </dl>
        <p class="text-xs text-muted-foreground">
          仅展示当前分配，不授予节点管理权限。每条流可能使用不同路径，单个会话状态不代表整个宿主。
        </p>
      </details>
    {/if}
    {#if view.error}<InlineNotice tone="danger">{view.error}</InlineNotice>{/if}
    {#if view.notice}<InlineNotice>{view.notice}</InlineNotice>{/if}
    <Button variant="outline" onclick={onrefresh} disabled={blocked}
      >刷新 Relay 状态</Button
    >
  </Card.Content>
</Card.Root>

<AlertDialog.Root
  open={confirmation !== null}
  onOpenChange={(open) => {
    if (!open) confirmation = null;
  }}
>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{confirmTitle}</AlertDialog.Title>
      <AlertDialog.Description>
        {#if confirmation === "pair" || confirmation === "rebind" || confirmation === "enable"}
          Relay 终止 TLS，能够读取转发内容，并作为登录信任方授权远程
          Shell。这不是端到端加密。仅信任已配置的控制服务，不向其提交恢复
          Token。
          {#if confirmation === "rebind"}重新绑定将撤销旧密钥和登录访问，保留固定公共地址。{/if}
        {:else if confirmation === "delete"}
          先停止访问，远端确认撤销后才删除本地绑定。无法确认时会保留绑定供显式重试。
        {:else}
          此操作会断开当前 Relay
          入口及其派生登录与优化连接；持久终端进程不会退出。当前公网页面可能失去连接。
        {/if}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>取消</AlertDialog.Cancel>
      <Button
        variant={confirmation === "delete" ? "destructive" : "default"}
        onclick={confirm}
        >确认{confirmation === "enable" ? "启用" : "操作"}</Button
      >
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
