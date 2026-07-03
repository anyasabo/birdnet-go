<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '$lib/i18n';
  import { fetchWithCSRF } from '$lib/utils/api';
  import type {
    DetectionsListData,
    DetectionQueryParams,
    DetectionSortBy,
  } from '$lib/types/detection.types';
  import DetectionsCard from './components/DetectionsCard.svelte';
  import { getLogger } from '$lib/utils/logger';
  import { getLocalDateString } from '$lib/utils/date';
  import { navigation } from '$lib/stores/navigation.svelte';
  import { parseDetectionQueryParams } from './detectionQuery';

  const logger = getLogger('app');

  let detectionsData = $state<DetectionsListData | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  // Local storage keys for user preferences
  const RESULTS_PER_PAGE_KEY = 'birdnet-detections-results-per-page';
  const SORT_BY_KEY = 'birdnet-detections-sort-by';

  // Get saved preference from localStorage
  function getSavedResultsPerPage(): number {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem(RESULTS_PER_PAGE_KEY);
      if (saved && !isNaN(parseInt(saved))) {
        const value = parseInt(saved);
        // Validate it's one of our allowed values
        if ([10, 25, 50, 100].includes(value)) {
          return value;
        }
      }
    }
    return 25; // Default
  }

  // Extract query parameters from URL. The parsing rules live in the pure
  // parseDetectionQueryParams helper (unit-tested); this reads the DOM/storage
  // inputs it needs and delegates.
  function getQueryParams(): DetectionQueryParams {
    return parseDetectionQueryParams(window.location.search, {
      savedResultsPerPage: getSavedResultsPerPage(),
      savedSortBy: typeof window !== 'undefined' ? localStorage.getItem(SORT_BY_KEY) : null,
    });
  }

  // Fetch detections data
  async function fetchDetections() {
    loading = true;
    error = null;

    try {
      const queryParams = getQueryParams();
      // Build query string
      const queryString = new URLSearchParams();
      Object.entries(queryParams).forEach(([key, value]) => {
        if (value !== undefined) {
          queryString.append(key, String(value));
        }
      });

      // Always include weather data for the detections page
      queryString.append('includeWeather', 'true');

      const data = (await fetchWithCSRF(`/api/v2/detections?${queryString.toString()}`)) as any;

      // Validate numResults before using
      const validatedNumResults =
        queryParams.numResults !== undefined && [10, 25, 50, 100].includes(queryParams.numResults)
          ? queryParams.numResults
          : getSavedResultsPerPage();

      // Transform API response to match our expected format
      const hasDateRange = Boolean(queryParams.start_date || queryParams.end_date);
      detectionsData = {
        notes: data.data || [],
        queryType: queryParams.queryType || 'all',
        // A range query owns date filtering; don't fall back to today (that would
        // render a misleading single-date title).
        date: queryParams.date?.trim() || (hasDateRange ? '' : getLocalDateString()),
        startDate: queryParams.start_date,
        endDate: queryParams.end_date,
        hour: queryParams.hour ? parseInt(queryParams.hour) : undefined,
        duration: queryParams.duration,
        species: queryParams.species,
        search: queryParams.search,
        numResults: validatedNumResults,
        offset: queryParams.offset!,
        totalResults: data.total || 0,
        itemsPerPage: data.limit || validatedNumResults,
        currentPage: data.current_page || 1,
        totalPages: data.total_pages || 1,
        showingFrom: (queryParams.offset || 0) + 1,
        showingTo: Math.min((queryParams.offset || 0) + (data.data?.length || 0), data.total || 0),
        dashboardSettings: data.dashboardSettings,
      };
    } catch (err) {
      error = err instanceof Error ? err.message : t('detections.errors.fetchFailed');
      logger.error('Error fetching detections:', err);
    } finally {
      loading = false;
    }
  }

  // Handle page change
  function handlePageChange(newPage: number) {
    if (detectionsData) {
      const newOffset = (newPage - 1) * detectionsData.itemsPerPage;
      const params = new URLSearchParams(window.location.search);
      params.set('offset', String(newOffset));

      // Update URL without navigation
      window.history.pushState({}, '', `${window.location.pathname}?${params.toString()}`);

      // Fetch new data
      fetchDetections();
    }
  }

  // Handle numResults change with debouncing
  function handleNumResultsChange(newNumResults: number) {
    // Save user preference to localStorage
    if (typeof window !== 'undefined') {
      localStorage.setItem(RESULTS_PER_PAGE_KEY, String(newNumResults));
    }

    // Clear existing timer
    if (debounceTimer) {
      clearTimeout(debounceTimer);
    }

    // Set loading state immediately for user feedback
    loading = true;

    // Debounce the actual fetch
    debounceTimer = setTimeout(() => {
      const params = new URLSearchParams(window.location.search);
      params.set('numResults', String(newNumResults));
      params.set('offset', '0'); // Reset to first page

      // Update URL without navigation
      window.history.pushState({}, '', `${window.location.pathname}?${params.toString()}`);

      // Fetch new data
      fetchDetections();
    }, 300); // 300ms debounce delay
  }

  // Handle sort change from DetectionsList
  function handleSortChange(newSortBy: DetectionSortBy) {
    // Save preference to localStorage
    if (typeof window !== 'undefined') {
      localStorage.setItem(SORT_BY_KEY, newSortBy);
    }

    const params = new URLSearchParams(window.location.search);
    if (newSortBy && newSortBy !== 'date_desc') {
      params.set('sortBy', newSortBy);
    } else {
      params.delete('sortBy');
    }
    params.set('offset', '0'); // Reset to first page

    window.history.pushState({}, '', `${window.location.pathname}?${params.toString()}`);
    fetchDetections();
  }

  // Handle details click
  function handleDetailsClick(id: number) {
    // Navigate to detection details page
    navigation.navigate(`/ui/detections/${id}`);
  }

  // Listen for search updates from SearchBox
  function handleSearchUpdate(event: Event) {
    const customEvent = event as CustomEvent<{ search: string }>;
    const { search } = customEvent.detail;
    // Update URL parameters to include new search
    const params = new URLSearchParams(window.location.search);
    if (search) {
      params.set('search', search);
    } else {
      params.delete('search');
    }

    // Update URL without navigation
    const url = new URL(window.location.href);
    url.search = params.toString();
    window.history.replaceState({}, '', url.toString());

    // Refresh detections with new search
    fetchDetections();
  }

  // Handle browser back/forward buttons
  function handlePopState() {
    fetchDetections();
  }

  onMount(() => {
    fetchDetections();

    // Listen for search updates from SearchBox
    window.addEventListener('searchUpdate', handleSearchUpdate);

    // Listen for browser navigation
    window.addEventListener('popstate', handlePopState);

    return () => {
      window.removeEventListener('searchUpdate', handleSearchUpdate);
      window.removeEventListener('popstate', handlePopState);

      // Clear any pending debounce timer
      if (debounceTimer) {
        clearTimeout(debounceTimer);
      }
    };
  });
</script>

<div class="col-span-12 space-y-6">
  <DetectionsCard
    data={detectionsData}
    {loading}
    {error}
    onPageChange={handlePageChange}
    onDetailsClick={handleDetailsClick}
    onRefresh={fetchDetections}
    onNumResultsChange={handleNumResultsChange}
    onSortChange={handleSortChange}
  />
</div>
