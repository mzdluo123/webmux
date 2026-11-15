<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
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
    label={m.s1_quick_start()}
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else}
  <main class="mx-auto max-w-3xl space-y-4 p-6">
    <h1 class="text-2xl font-semibold">{m.s1_new_chat()}</h1>
    <p class="text-sm text-muted-foreground">
      {m.s1_quick_start_no_workspace_desc()}
    </p>
    <LazyPage
      load={() => import("../components/sessions/NewSessionDialog.svelte")}
      label={m.s1_create_chat()}
    >
      {#snippet children(page)}
        <page.default
          launchConfigs={shell.launchConfigs?.state ?? {
            status: "unavailable",
            error: m.s1_launch_configs_unavailable(),
          }}
          defaultCwd={shell.setup.value.cwd}
          repositories={[]}
          onchange={() => void shell.launchConfigs?.load()}
          onworktreecreated={() => {
            throw new Error(m.s1_workspace_feature_unavailable());
          }}
          onclose={() => navigate("/")}
        />
      {/snippet}
    </LazyPage>
  </main>
{/if}
