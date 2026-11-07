<!--
  SpeciesLink.svelte

  Wraps a species name in a link to that species' detections. A real
  anchor, so open-in-new-tab and copy-link work, with SPA navigation on a plain
  click. Renders the content unlinked when there is no scientific name to link to.

  Props:
  - scientificName: string | null | undefined - Species to link to
  - range?: SpeciesDateRange - Date scope of the link; all dates when omitted
  - className?: string - Additional CSS classes
  - children: Snippet - Link content
-->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLAnchorAttributes } from 'svelte/elements';
  import { handleAppLinkClick } from '$lib/stores/navigation.svelte';
  import { cn } from '$lib/utils/cn';
  import { buildSpeciesRangeDetectionUrl, type SpeciesDateRange } from '$lib/utils/detectionUrls';

  interface Props extends Omit<HTMLAnchorAttributes, 'href' | 'class' | 'onclick'> {
    scientificName: string | null | undefined;
    range?: SpeciesDateRange;
    className?: string;
    children: Snippet;
  }

  let { scientificName, range, className = '', children, ...rest }: Props = $props();
</script>

{#if scientificName}
  <a
    href={buildSpeciesRangeDetectionUrl(scientificName, range)}
    class={cn('hover:text-[var(--color-primary)] hover:underline', className)}
    onclick={handleAppLinkClick}
    {...rest}
  >
    {@render children()}
  </a>
{:else}
  <span class={className}>{@render children()}</span>
{/if}
