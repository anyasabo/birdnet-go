import type { DetectionQueryParams, DetectionSortBy } from '$lib/types/detection.types';
import { getLocalDateString } from '$lib/utils/date';

/**
 * Sort values the detections list accepts from the URL or localStorage.
 * Anything else is ignored (falls back to the stored/default sort).
 */
export const ALLOWED_SORT_VALUES = new Set<string>([
  'date_desc',
  'date_asc',
  'species_asc',
  'species_desc',
  'confidence_asc',
  'confidence_desc',
  'status',
]);

const ALLOWED_RESULTS_PER_PAGE = [10, 25, 50, 100];

/**
 * Inputs the pure parser needs that would otherwise come from the DOM/storage.
 * The Svelte component reads window/localStorage and passes them in, keeping this
 * function pure and unit-testable.
 */
export interface ParseDetectionQueryOptions {
  /** Fallback results-per-page when the URL omits/invalidates numResults. */
  savedResultsPerPage: number;
  /** Persisted sortBy (from localStorage); used only when the URL omits sortBy. */
  savedSortBy?: string | null;
  /** Today's local date; injectable so tests are deterministic. Defaults to today. */
  today?: string;
}

/**
 * Parse the detections-list query parameters from a URL search string.
 *
 * Extracted from DetectionsPage so the parsing rules — especially the
 * start_date/end_date range handling that must NOT collapse to a single `date` —
 * can be unit-tested without mounting the component.
 */
export function parseDetectionQueryParams(
  search: string,
  opts: ParseDetectionQueryOptions
): DetectionQueryParams {
  const params = new URLSearchParams(search);

  // Trimmed param value, or undefined when the param is absent/blank. URL params
  // may be present-but-empty (?x=), which we treat as absent.
  const value = (key: string): string | undefined => {
    const v = params.get(key)?.trim();
    return v === undefined || v.length === 0 ? undefined : v;
  };

  const searchTerm = value('search');

  // Set queryType to 'search' when only a search parameter is present, else 'all'.
  const queryType =
    (value('queryType') as DetectionQueryParams['queryType'] | undefined) ??
    (searchTerm ? 'search' : 'all');

  // Parse and validate numResults; fall back to the saved preference.
  const numResultsParam = value('numResults');
  let numResults = numResultsParam ? parseInt(numResultsParam) : opts.savedResultsPerPage;
  if (isNaN(numResults) || !ALLOWED_RESULTS_PER_PAGE.includes(numResults)) {
    numResults = opts.savedResultsPerPage;
  }

  // Parse and validate sortBy from URL, then fall back to the persisted value.
  const sortByParam = value('sortBy');
  const savedSortBy = opts.savedSortBy ?? undefined;
  let sortBy: DetectionSortBy | undefined;
  if (sortByParam && ALLOWED_SORT_VALUES.has(sortByParam)) {
    sortBy = sortByParam as DetectionSortBy;
  } else if (savedSortBy && ALLOWED_SORT_VALUES.has(savedSortBy)) {
    sortBy = savedSortBy as DetectionSortBy;
  }

  // A start_date/end_date pair spans multiple days (e.g. jumping to a species
  // from the analytics pages). The backend routes these through advanced search
  // and honors the range, so we must not also pin a single `date`.
  const startDate = value('start_date');
  const endDate = value('end_date');
  const hasDateRange = Boolean(startDate ?? endDate);

  // Only default to today's date for non-search query types.
  // For search queries, omitting the date allows searching across all dates.
  // When date is included, the backend restricts results to that single day,
  // which causes search to return no results for species detected on other days.
  // A date range takes precedence: leave `date` unset so the range owns filtering.
  const today = opts.today ?? getLocalDateString();
  const explicitDate = value('date');
  const date = explicitDate ?? (queryType !== 'search' && !hasDateRange ? today : undefined);

  const durationParam = value('duration');
  const offsetParam = value('offset');

  return {
    queryType,
    date,
    hour: value('hour'),
    duration: durationParam ? parseInt(durationParam) : undefined,
    species: value('species'),
    search: searchTerm,
    start_date: startDate,
    end_date: endDate,
    numResults,
    offset: offsetParam ? parseInt(offsetParam) : 0,
    sortBy,
  };
}
