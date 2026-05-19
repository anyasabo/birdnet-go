<script lang="ts">
  import type { DetectionsListData, DetectionSortBy } from '$lib/types/detection.types';
  import DetectionsList from './DetectionsList.svelte';

  interface Props {
    data: DetectionsListData | null;
    loading?: boolean;
    error?: string | null;
    onPageChange: (_newPage: number) => void;
    onDetailsClick: (_id: number) => void;
    onRefresh: () => void;
    onNumResultsChange: (_numResults: number) => void;
    onSortChange?: (_sortBy: DetectionSortBy) => void;
    onModelVersionChange?: (_version: string) => void;
    modelVersion?: string;
  }

  let {
    data = null,
    loading = false,
    error = null,
    onPageChange,
    onDetailsClick,
    onRefresh,
    onNumResultsChange,
    onSortChange,
    onModelVersionChange,
    modelVersion = '',
  }: Props = $props();
</script>

<section class="card col-span-12 bg-[var(--color-base-100)] shadow-xs">
  <div class="card-body grow-0 p-2 sm:p-4 sm:pt-3">
    <DetectionsList
      {data}
      {loading}
      {error}
      {onPageChange}
      {onDetailsClick}
      {onRefresh}
      {onNumResultsChange}
      {onSortChange}
      {onModelVersionChange}
      {modelVersion}
    />
  </div>
</section>
