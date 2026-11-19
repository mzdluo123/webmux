<script lang="ts">
  import { onMount, tick } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import * as NavigationMenu from "$lib/components/ui/navigation-menu";
  import { observeStickyHeaderOffset } from "../../hooks/shared/sticky-header";

  let { sections }: { sections: readonly { id: string; label: string }[] } =
    $props();
  let active = $state("appearance");
  let navigation: HTMLElement;
  let vertical = $state(false);
  let links: HTMLDivElement | null;
  // Which edges of the horizontal strip hide more links; they fade out.
  let more = $state({ start: false, end: false });

  function edges() {
    if (!links) return;
    more = {
      start: links.scrollLeft > 1,
      end: links.scrollLeft + links.clientWidth < links.scrollWidth - 1,
    };
  }

  onMount(() => {
    const targets = sections.flatMap(({ id }) => {
      const target = document.getElementById(id);
      return target ? [target] : [];
    });
    const desktop = matchMedia("(min-width: 1024px)");
    let frame = 0;
    let restoreReading = false;
    let width = innerWidth;
    let reading:
      | { section: HTMLElement; offset: number; atEnd: boolean }
      | undefined;

    function update() {
      frame = 0;
      if (width !== innerWidth) {
        width = innerWidth;
        restoreReading = true;
      }
      const nextVertical = desktop.matches;
      if (vertical !== nextVertical) {
        vertical = nextVertical;
        restoreReading = true;
        // The new orientation can change the navigation height.
        void tick().then(() => {
          if (navigation?.isConnected && !frame) update();
        });
        return;
      }
      const first = targets[0];
      if (!first || !links) return;
      const threshold =
        Math.max(
          parseFloat(getComputedStyle(first).scrollMarginTop),
          nextVertical
            ? parseFloat(getComputedStyle(navigation).top)
            : navigation.getBoundingClientRect().bottom,
        ) + 1;
      // The navigation changes which part of the page stays visible.
      // Keep the current section at the same offset from that boundary.
      if (restoreReading && reading?.section.isConnected) {
        const focused = document.activeElement === reading.section;
        const followEnd = reading.atEnd && !focused;
        // Scroll positions round to CSS pixels. Stay on the active side
        // of the threshold after alignment.
        const offset = focused ? -1 : Math.min(reading.offset, -1);
        const delta = followEnd
          ? document.documentElement.scrollHeight
          : reading.section.getBoundingClientRect().top - threshold - offset;
        if (Math.abs(delta) > 1)
          window.scrollBy({ top: delta, behavior: "instant" });
      }
      restoreReading = false;
      let index = 0;
      for (const [i, target] of targets.entries()) {
        if (target.getBoundingClientRect().top <= threshold) index = i;
      }
      const bottom = document.documentElement.scrollHeight - innerHeight;
      const atEnd = bottom > 0 && scrollY >= bottom - 2;
      if (atEnd) index = targets.length - 1;
      const current = targets[index]!;
      active = current.id;
      reading =
        scrollY > 0
          ? {
              section: current,
              offset: current.getBoundingClientRect().top - threshold,
              atEnd,
            }
          : undefined;
      // Scroll only the horizontal strip; scrollIntoView would move the page.
      const link = links.querySelector<HTMLAnchorElement>(
        `a[href="#${active}"]`,
      );
      if (!link) return;
      const item = link.getBoundingClientRect();
      const strip = links.getBoundingClientRect();
      if (item.left < strip.left) links.scrollLeft -= strip.left - item.left;
      else if (item.right > strip.right)
        links.scrollLeft += item.right - strip.right;
      edges();
    }
    function schedule() {
      if (!frame) frame = requestAnimationFrame(update);
    }
    function followHash() {
      const hash =
        location.hash === "#agent-tools" ? "#launch-configs" : location.hash;
      const target = targets.find((element) => `#${element.id}` === hash);
      if (target) {
        target.scrollIntoView({ block: "start" });
        target.focus({ preventScroll: true });
      }
      schedule();
    }

    for (const target of targets) target.tabIndex = -1;
    const stopOffset = observeStickyHeaderOffset((height) => {
      navigation.style.top = height ? `${height}px` : "";
      schedule();
    });
    const observer = new ResizeObserver(schedule);
    observer.observe(document.getElementById("settings-content")!);
    observer.observe(navigation);
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    window.addEventListener("hashchange", followHash);
    followHash();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      stopOffset();
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      window.removeEventListener("hashchange", followHash);
    };
  });
</script>

<div bind:this={navigation} class="settings-navigation">
  <p
    class="mb-3 hidden px-3 text-xs font-medium text-muted-foreground lg:block"
  >
    {m.s2_settings_sections()}
  </p>
  <div
    bind:this={links}
    class="settings-links"
    data-more-start={more.start ? "" : undefined}
    data-more-end={more.end ? "" : undefined}
    onscroll={edges}
  >
    <NavigationMenu.Root
      aria-label={m.s2_settings_sections()}
      layout="sections"
      viewport={false}
      orientation={vertical ? "vertical" : "horizontal"}
    >
      <NavigationMenu.List layout="sections">
        {#each sections as section (section.id)}
          <NavigationMenu.Item>
            <NavigationMenu.Link
              href={`#${section.id}`}
              variant="section"
              active={active === section.id}
              aria-current={active === section.id ? "location" : undefined}
              >{section.label}</NavigationMenu.Link
            >
          </NavigationMenu.Item>
        {/each}
      </NavigationMenu.List>
    </NavigationMenu.Root>
  </div>
</div>

<style>
  .settings-navigation {
    position: sticky;
    top: 0;
    z-index: 10;
    min-width: 0;
    align-self: start;
    padding-block: 0.75rem;
    border-bottom: 1px solid var(--border);
    background: var(--background);
  }
  .settings-links {
    overflow-x: auto;
    padding: 0.25rem;
    scrollbar-width: thin;
  }
  .settings-links[data-more-start] {
    mask-image: linear-gradient(to right, transparent, #000 1.75rem);
  }
  .settings-links[data-more-end] {
    mask-image: linear-gradient(to left, transparent, #000 1.75rem);
  }
  .settings-links[data-more-start][data-more-end] {
    mask-image: linear-gradient(
      to right,
      transparent,
      #000 1.75rem,
      #000 calc(100% - 1.75rem),
      transparent
    );
  }
  @media (min-width: 1024px) {
    .settings-links {
      mask-image: none !important;
    }
    .settings-navigation {
      top: 1rem;
      margin-top: 2rem;
      border-bottom: 0;
    }
  }
</style>
