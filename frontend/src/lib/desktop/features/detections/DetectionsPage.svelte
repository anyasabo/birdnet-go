<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { t } from '$lib/i18n';
  import { fetchWithCSRF } from '$lib/utils/api';
  import type {
    DetectionsListData,
    DetectionQueryParams,
    DetectionSortBy,
    PaginatedDetectionResponse,
  } from '$lib/types/detection.types';
  import DetectionsCard from './components/DetectionsCard.svelte';
  import { getLogger } from '$lib/utils/logger';
  import { getLocalDateString } from '$lib/utils/date';
  import { parseDetectionDateScope } from '$lib/utils/detectionUrls';
  import { navigation } from '$lib/stores/navigation.svelte';

  const logger = getLogger('app');

  let detectionsData = $state<DetectionsListData | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  // Local storage keys for user preferences
  const RESULTS_PER_PAGE_KEY = 'birdnet-detections-results-per-page';
  const SORT_BY_KEY = 'birdnet-detections-sort-by';

  const ALLOWED_SORT_VALUES = new Set<string>([
    'date_desc',
    'date_asc',
    'species_asc',
    'species_desc',
    'confidence_asc',
    'confidence_desc',
    'status',
  ]);

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

  // Extract query parameters from URL
  function getQueryParams(): DetectionQueryParams {
    const params = new URLSearchParams(window.location.search);
    const search = params.get('search');

    // Set queryType to 'search' if search parameter is present
    let queryType = params.get('queryType') as DetectionQueryParams['queryType'];
    if (search && !queryType) {
      queryType = 'search';
    } else if (!queryType) {
      queryType = 'all';
    }

    // Parse and validate numResults
    const numResultsParam = params.get('numResults');
    let numResults = numResultsParam ? parseInt(numResultsParam) : getSavedResultsPerPage();

    // Validate numResults is one of allowed values
    if (isNaN(numResults) || ![10, 25, 50, 100].includes(numResults)) {
      numResults = getSavedResultsPerPage();
    }

    // Parse and validate sortBy from URL or localStorage
    const sortByParam = params.get('sortBy');
    let sortBy: DetectionSortBy | undefined;
    if (sortByParam && ALLOWED_SORT_VALUES.has(sortByParam)) {
      sortBy = sortByParam as DetectionSortBy;
    } else if (typeof window !== 'undefined') {
      const saved = localStorage.getItem(SORT_BY_KEY);
      if (saved && ALLOWED_SORT_VALUES.has(saved)) {
        sortBy = saved as DetectionSortBy;
      }
    }

    const dateScope = parseDetectionDateScope(params, queryType, getLocalDateString());

    const durationParam = params.get('duration');

    return {
      queryType,
      ...dateScope,
      hour: params.get('hour') || undefined,
      duration: durationParam ? parseInt(durationParam) : undefined,
      species: params.get('species') || undefined,
      search: search || undefined,
      numResults,
      offset: parseInt(params.get('offset') || '0'),
      sortBy,
    };
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

      const data = await fetchWithCSRF<PaginatedDetectionResponse>(
        `/api/v2/detections?${queryString.toString()}`
      );

      // Validate numResults before using
      const validatedNumResults =
        queryParams.numResults !== undefined && [10, 25, 50, 100].includes(queryParams.numResults)
          ? queryParams.numResults
          : getSavedResultsPerPage();

      // Transform API response to match our expected format
      detectionsData = {
        notes: data.data || [],
        queryType: queryParams.queryType || 'all',
        date: queryParams.date?.trim() || getLocalDateString(),
        filterDate: queryParams.date,
        startDate: queryParams.start_date,
        endDate: queryParams.end_date,
        hour: queryParams.hour ? parseInt(queryParams.hour) : undefined,
        duration: queryParams.duration,
        species: queryParams.species,
        search: queryParams.search,
        numResults: validatedNumResults,
        offset: queryParams.offset ?? 0,
        totalResults: data.total || 0,
        itemsPerPage: data.limit || validatedNumResults,
        currentPage: data.current_page || 1,
        totalPages: data.total_pages || 1,
        showingFrom: (queryParams.offset || 0) + 1,
        showingTo: Math.min((queryParams.offset || 0) + (data.data?.length || 0), data.total || 0),
      };
    } catch (err) {
      error = err instanceof Error ? err.message : t('detections.errors.fetchFailed');
      logger.error('Error fetching detections:', err);
    } finally {
      loading = false;
    }
  }

  // The store's query string is the only fetch trigger, so links, back/forward and
  // this page's own controls all refresh the list the same way.
  const listPath = untrack(() => navigation.currentPath);
  $effect(() => {
    void navigation.currentSearch;
    // Leaving the page changes the search too; that is not a query for this list.
    if (navigation.currentPath === listPath) untrack(() => fetchDetections());
  });

  // Puts a new query in the URL, which refreshes the list through the effect above.
  function applyQuery(params: URLSearchParams) {
    const search = `?${params.toString()}`;
    if (search === window.location.search) {
      fetchDetections();
    } else {
      navigation.navigate(`${window.location.pathname}${search}`);
    }
  }

  // Handle page change
  function handlePageChange(newPage: number) {
    if (detectionsData) {
      const newOffset = (newPage - 1) * detectionsData.itemsPerPage;
      const params = new URLSearchParams(window.location.search);
      params.set('offset', String(newOffset));

      applyQuery(params);
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

      applyQuery(params);
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

    applyQuery(params);
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

    const query = params.toString();
    navigation.redirect(`${window.location.pathname}${query ? `?${query}` : ''}`);
  }

  onMount(() => {
    // Listen for search updates from SearchBox
    window.addEventListener('searchUpdate', handleSearchUpdate);

    return () => {
      window.removeEventListener('searchUpdate', handleSearchUpdate);

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
