<script lang="ts">
  import { onMount } from "svelte";
  import SettingsPageHeader from "../components/settings/SettingsPageHeader.svelte";
  import RelayManager from "../components/settings/RelayManager.svelte";
  import { setBrowserChrome } from "../platform/browser-chrome";
  import { api } from "../platform/api";
  import { NoContentSchema } from "../contracts/schemas";
  import { clearRelayAuth } from "../platform/relay-auth";
  import { relayAuth } from "../platform/relay-auth.svelte";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import RelayPathStatus from "../components/settings/RelayPathStatus.svelte";

  let logoutError = $state("");
  let loggingOut = $state(false);
  async function logout() {
    if (loggingOut) return;
    loggingOut = true;
    logoutError = "";
    window.dispatchEvent(new Event("webmux:relay-revoked"));
    try {
      await api("/relay/auth/logout", NoContentSchema, "POST");
      clearRelayAuth();
      window.dispatchEvent(new Event("webmux:auth"));
    } catch {
      logoutError = "无法确认注销，请重试；不要将本机退出视为控制台全局注销。";
    } finally {
      loggingOut = false;
    }
  }
  onMount(() => {
    setBrowserChrome("WebMux · 官方 Relay", false);
  });
</script>

<div class="safe-area-page min-h-dvh settings-layout">
  <div class="col-span-full"><SettingsPageHeader /></div>
  <main class="mx-auto w-full max-w-3xl space-y-6 px-4 py-8 sm:px-8 sm:py-10">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">
        官方 Relay
      </h1>
      <p class="mt-2 text-sm text-muted-foreground">
        GitHub 配对后，在其他设备登录同一账号的所有者控制台，即可打开在线
        WebMux，无需复制 Token。
      </p>
    </div>
    <RelayManager />
    <RelayPathStatus />
    {#if relayAuth.authenticated}
      <section class="space-y-3" aria-label="当前 Relay 登录">
        <h2 class="text-base font-semibold">当前浏览器的 Relay 登录</h2>
        <p class="text-sm text-muted-foreground">
          注销只关闭当前宿主访问，不结束持久终端。控制台全局注销请在所有者控制台操作；GitHub
          网站注销不等于 WebMux 注销。
        </p>
        <Button
          variant="outline"
          disabled={loggingOut}
          onclick={() => void logout()}>注销当前宿主登录</Button
        >
        {#if logoutError}<InlineNotice tone="danger">{logoutError}</InlineNotice
          >{/if}
      </section>
    {/if}
  </main>
</div>
