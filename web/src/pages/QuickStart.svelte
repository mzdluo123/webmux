<script lang="ts">
  import { getShell } from "../hooks/shared/shell.svelte";
  import { pluginAvailable } from "../platform/plugin-host.svelte";
  import { navigate } from "../platform/api";
  import LazyPage from "../components/shared/LazyPage.svelte";
  const shell = getShell();
  const workspaceAvailable = $derived(
    pluginAvailable("workspace.manage") && !!shell.repositories,
  );
</script>

{#if workspaceAvailable}
  <LazyPage
    load={() => import("./WorkspaceQuickStart.svelte")}
    label="快速开始"
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else}
  <main class="mx-auto max-w-3xl space-y-4 p-6">
    <h1 class="text-2xl font-semibold">新聊天</h1>
    <p class="text-sm text-muted-foreground">
      在已有目录中选择聊天启动配置，无需登记仓库或创建工作树。
    </p>
    <LazyPage
      load={() => import("../components/sessions/NewSessionDialog.svelte")}
      label="创建聊天"
    >
      {#snippet children(page)}
        <page.default
          launchConfigs={shell.launchConfigs?.state ?? {
            status: "unavailable",
            error: "启动配置界面暂不可用",
          }}
          defaultCwd={shell.setup.value.cwd}
          repositories={[]}
          onchange={() => void shell.launchConfigs?.load()}
          onworktreecreated={() => {
            throw new Error("工作区功能暂不可用");
          }}
          onclose={() => navigate("/")}
        />
      {/snippet}
    </LazyPage>
  </main>
{/if}
