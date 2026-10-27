<script lang="ts">
  // Context-window usage from `usage_update` (`used / size`).
  let {
    used,
    size,
    class: className,
  }: { used: number; size: number; class?: string } = $props();
  const ratio = $derived(Math.min(1, used / size));
  const circumference = 2 * Math.PI * 15;
</script>

<svg
  viewBox="0 0 36 36"
  class={["shrink-0 -rotate-90", className]}
  role="img"
  aria-label={`上下文已用 ${Math.round(ratio * 100)}%`}
>
  <circle
    cx="18"
    cy="18"
    r="15"
    fill="none"
    class="stroke-muted"
    stroke-width="4"
  />
  <circle
    cx="18"
    cy="18"
    r="15"
    fill="none"
    class={ratio > 0.8 ? "stroke-warning" : "stroke-primary"}
    stroke-width="4"
    stroke-linecap="round"
    stroke-dasharray={`${ratio * circumference} ${circumference}`}
  />
</svg>
