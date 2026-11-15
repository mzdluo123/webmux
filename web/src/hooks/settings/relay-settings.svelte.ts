import { onMount } from "svelte";
import { api, APIError } from "../../platform/api";
import { NoContentSchema } from "../../contracts/schemas";
import {
  RelayStatusSchema,
  RelayEnrollmentSchema,
  RelayEnrollmentRequestSchema,
  RelayPollRequestSchema,
  RelayPollResultSchema,
  RelayUpdateSchema,
  type RelayStatus,
  type RelayEnrollment,
} from "../../contracts/relay-schemas";
import { clearRelayAuth } from "../../platform/relay-auth";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface RelaySettingsView {
  status: RelayStatus | undefined;
  enrollment: RelayEnrollment | undefined;
  loading: boolean;
  busy: boolean;
  error: string;
  notice: string;
}

function failureMessage(cause: unknown) {
  if (!(cause instanceof APIError)) return "无法连接宿主；请刷新重试。";
  if (cause.notPersisted)
    return "访问已停止，但关闭状态未持久化；修复存储后重新保存。";
  switch (cause.status) {
    case 401:
      return "登录已失效，请重新登录。";
    case 403:
      return "授权被拒绝；请检查账号邀请、所有者身份与当前入口权限。";
    case 409:
      return "绑定或操作状态已变化；请刷新后显式重新操作。";
    case 410:
      return "配对已过期，请重新开始。";
    case 429:
      return "操作过于频繁，请稍后重新操作。";
    case 503:
      return "控制服务或已分配节点暂不可用；WSS 就绪前不会开放入口。";
    default:
      return "操作未完成；请刷新确认实际状态，不要重复未知结果的操作。";
  }
}

export function useRelaySettings() {
  const ui = $state<RelaySettingsView>({
    status: undefined,
    enrollment: undefined,
    loading: true,
    busy: false,
    error: "",
    notice: "",
  });
  const requests = new LatestRequest();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pairingTimer: ReturnType<typeof setTimeout> | undefined;
  let pairingRevision = 0;
  let mounted = false;

  function accept(status: RelayStatus) {
    const previous = ui.status;
    ui.status = status;
    if (
      previous &&
      (!status.paired ||
        !status.enabled ||
        (previous.controlReady && !status.controlReady))
    )
      clearRelayAuth();
  }

  async function refresh() {
    if (ui.busy || requests.disposed) return;
    const ticket = requests.begin();
    try {
      const status = await api(
        "/relay",
        RelayStatusSchema,
        "GET",
        undefined,
        ticket.signal,
      );
      if (ticket.current()) accept(status);
    } catch (cause) {
      if (ticket.reportable()) ui.error = failureMessage(cause);
    } finally {
      if (ticket.current()) ui.loading = false;
    }
  }

  function clearPairing() {
    pairingRevision++;
    clearTimeout(pairingTimer);
    ui.enrollment = undefined;
  }

  async function pollEnrollment(enrollment: RelayEnrollment, revision: number) {
    if (!mounted || revision !== pairingRevision) return;
    if (Date.now() >= enrollment.expiresAt * 1000) {
      clearPairing();
      ui.error = "配对已过期，请重新开始。";
      return;
    }
    const ticket = requests.begin();
    try {
      const result = await api(
        "/relay/enrollments/poll",
        RelayPollResultSchema,
        "POST",
        { handle: enrollment.handle },
        ticket.signal,
        RelayPollRequestSchema,
      );
      if (!ticket.current() || revision !== pairingRevision) return;
      if ("enabled" in result) {
        clearPairing();
        accept(result);
        ui.notice = "GitHub 主机配对已完成；公网访问仍需单独启用。";
        return;
      }
      pairingTimer = setTimeout(
        () => void pollEnrollment(enrollment, revision),
        enrollment.pollIntervalSeconds * 1000,
      );
    } catch (cause) {
      if (!ticket.reportable() || revision !== pairingRevision) return;
      clearPairing();
      ui.error = failureMessage(cause);
    }
  }

  async function pair(rebind: boolean) {
    if (ui.busy || ui.enrollment || requests.disposed) return;
    ui.busy = true;
    ui.error = "";
    ui.notice = "";
    const revision = ++pairingRevision;
    const ticket = requests.begin();
    try {
      const enrollment = await api(
        "/relay/enrollments",
        RelayEnrollmentSchema,
        "POST",
        { rebind, confirmPublic: true },
        ticket.signal,
        RelayEnrollmentRequestSchema,
      );
      if (!ticket.current() || revision !== pairingRevision) return;
      if (rebind) clearRelayAuth();
      ui.enrollment = enrollment;
      pairingTimer = setTimeout(
        () => void pollEnrollment(enrollment, revision),
        enrollment.pollIntervalSeconds * 1000,
      );
    } catch (cause) {
      if (ticket.reportable()) ui.error = failureMessage(cause);
    } finally {
      if (ticket.current()) ui.busy = false;
    }
  }

  async function cancelPairing() {
    const enrollment = ui.enrollment;
    clearPairing();
    requests.cancel();
    if (!enrollment || requests.disposed) return;
    ui.busy = true;
    const ticket = requests.begin();
    try {
      await api(
        "/relay/enrollments",
        NoContentSchema,
        "DELETE",
        { handle: enrollment.handle },
        ticket.signal,
        RelayPollRequestSchema,
      );
      if (ticket.current()) ui.notice = "配对已取消。";
    } catch (cause) {
      if (ticket.reportable()) ui.error = failureMessage(cause);
    } finally {
      if (ticket.current()) ui.busy = false;
    }
  }

  async function mutate(path: string, method: string, body: unknown) {
    if (ui.busy || requests.disposed) return;
    clearPairing();
    ui.busy = true;
    ui.error = "";
    ui.notice = "";
    window.dispatchEvent(new Event("webmux:relay-revoked"));
    const ticket = requests.begin();
    try {
      const result = await api(
        path,
        RelayPollResultSchema,
        method,
        body,
        ticket.signal,
      );
      if (!ticket.current()) return;
      if ("enabled" in result) {
        accept(result);
        ui.notice = "操作已完成；运行状态以当前就绪信息为准。";
      } else {
        ui.notice =
          "远端撤销尚未确认；本地绑定保留，请稍后刷新并显式重试删除。";
      }
    } catch (cause) {
      if (ticket.reportable()) ui.error = failureMessage(cause);
    } finally {
      if (ticket.current()) {
        ui.busy = false;
        await refresh();
      }
    }
  }

  onMount(() => {
    mounted = true;
    const invalidate = () => {
      clearPairing();
      requests.cancel();
      ui.busy = false;
      ui.status = undefined;
      ui.error = "登录已失效，请重新登录。";
    };
    window.addEventListener("webmux:auth", invalidate);
    async function poll() {
      if (!ui.enrollment && !document.hidden) await refresh();
      if (mounted) timer = setTimeout(() => void poll(), 2000);
    }
    void poll();
    return () => {
      mounted = false;
      window.removeEventListener("webmux:auth", invalidate);
      const enrollment = ui.enrollment;
      clearPairing();
      clearTimeout(timer);
      requests.dispose();
      if (enrollment)
        void api(
          "/relay/enrollments",
          NoContentSchema,
          "DELETE",
          { handle: enrollment.handle },
          undefined,
          RelayPollRequestSchema,
        ).catch(() => undefined);
    };
  });

  return {
    ui,
    refresh,
    pair,
    cancelPairing,
    setEnabled: (enabled: boolean) =>
      mutate(
        "/relay",
        "PUT",
        RelayUpdateSchema.parse({ enabled, confirmPublic: enabled }),
      ),
    disconnect: () => mutate("/relay/disconnect", "POST", {}),
    reconnect: () => mutate("/relay/reconnect", "POST", {}),
    deleteBinding: () => mutate("/relay/binding", "DELETE", {}),
  };
}
