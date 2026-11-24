<script lang="ts">
  import type { Snippet } from "svelte";
  import Brand from "./Brand.svelte";
  import LocaleChoice from "../shared/LocaleChoice.svelte";
  import * as Card from "$lib/components/ui/card";

  let {
    title,
    subtitle,
    children,
  }: {
    title: string;
    subtitle: string;
    children: Snippet;
  } = $props();
</script>

<main class="flex min-h-dvh flex-col items-center justify-center gap-4 p-6">
  <Card.Root class="w-full max-w-md" data-auth-gate-card>
    <Card.Header>
      <p class="brand mb-6" data-auth-gate-brand><Brand /></p>
      <Card.Title class="text-2xl"><h1>{title}</h1></Card.Title>
      <Card.Description>{subtitle}</Card.Description>
    </Card.Header>
    <Card.Content>{@render children()}</Card.Content>
  </Card.Root>
  <LocaleChoice />
</main>
