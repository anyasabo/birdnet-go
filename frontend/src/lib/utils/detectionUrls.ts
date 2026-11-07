/**
 * Detection URL builders for the dashboard.
 *
 * These functions wrap buildAppUrl so every anchor the dashboard emits carries
 * the current basepath (handles YAML webserver.basepath, X-Forwarded-Prefix, and
 * Home Assistant X-Ingress-Path). Extracted from DailySummaryCard.svelte which
 * previously built these URLs inline without the basepath wrap, causing
 * hourly-grid and species-row anchors to 404 through reverse proxies when the
 * DB had detection rows.
 */

import type { DetectionsListData } from '$lib/types/detection.types';
import { buildAppUrl } from './urlHelpers';

/** Inclusive date scope of a species link. The backend needs both bounds; one alone is dropped. */
export interface SpeciesDateRange {
  startDate?: string | null;
  endDate?: string | null;
}

/**
 * Builds the detail-page URL of one detection.
 */
export function buildDetectionDetailUrl(id: number | string): string {
  return buildAppUrl(`/ui/detections/${id}`);
}

/**
 * Builds a detection-list URL for a species across a date range, or across all
 * dates when no range is given. Used wherever a species is shown without a single
 * day of context; buildSpeciesDetectionUrl pins one day.
 */
export function buildSpeciesRangeDetectionUrl(
  scientificName: string,
  range?: SpeciesDateRange
): string {
  const params = new URLSearchParams({ queryType: 'species', species: scientificName });
  if (range?.startDate && range.endDate) {
    params.set('start_date', range.startDate);
    params.set('end_date', range.endDate);
  }
  return buildAppUrl(`/ui/detections?${params.toString()}`);
}

/**
 * Reads the date scope of a detection-list URL. A full range replaces the single date.
 * Search and species lists span all dates unless the URL narrows them; every other
 * list defaults to today.
 */
export function parseDetectionDateScope(
  params: URLSearchParams,
  queryType: string,
  today: string
): { date?: string; start_date?: string; end_date?: string } {
  const value = (key: string) => {
    const trimmed = params.get(key)?.trim();
    if (!trimmed) return undefined;
    return trimmed;
  };
  const start_date = value('start_date');
  const end_date = value('end_date');
  if (start_date && end_date) return { start_date, end_date };

  const spansAllDates = queryType === 'search' || queryType === 'species';
  return { date: value('date') ?? (spansAllDates ? undefined : today) };
}

/**
 * Builds the batch-resolve body for "select all matching". It must describe the
 * same query as the list on screen, because the resolved IDs feed bulk delete,
 * review and lock.
 */
export function buildBatchResolveQuery(data: DetectionsListData) {
  return {
    queryType: data.queryType,
    species: data.species,
    date: data.filterDate,
    startDate: data.startDate,
    endDate: data.endDate,
    search: data.search,
    hour: data.hour !== undefined ? String(data.hour) : undefined,
    duration: data.duration,
  };
}

/**
 * Builds a detection-list URL filtered to one hour (or a range) on the given date.
 * Used by the dashboard hourly-grid cells at one, two, and six hour groupings.
 */
export function buildHourlyDetectionUrl(
  date: string,
  hour: number,
  durationHours: number,
  numResults?: number,
  offset?: number
): string {
  const params = new URLSearchParams({
    queryType: 'hourly',
    date,
    hour: hour.toString(),
    duration: durationHours.toString(),
  });
  if (numResults !== undefined) params.set('numResults', numResults.toString());
  if (offset !== undefined) params.set('offset', offset.toString());
  return buildAppUrl(`/ui/detections?${params.toString()}`);
}

/**
 * Builds a detection-list URL filtered to all detections of a species on a date.
 * Used by the dashboard species-row anchors.
 */
export function buildSpeciesDetectionUrl(
  scientificName: string,
  date: string,
  numResults?: number,
  offset?: number
): string {
  const params = new URLSearchParams({
    queryType: 'species',
    species: scientificName,
    date,
  });
  if (numResults !== undefined) params.set('numResults', numResults.toString());
  if (offset !== undefined) params.set('offset', offset.toString());
  return buildAppUrl(`/ui/detections?${params.toString()}`);
}

/**
 * Builds a detection-list URL filtered to a species within a specific hour window.
 * Used by the dashboard per-species hourly cells.
 */
export function buildSpeciesHourUrl(
  scientificName: string,
  date: string,
  hour: number,
  durationHours: number,
  numResults?: number,
  offset?: number
): string {
  const params = new URLSearchParams({
    queryType: 'species',
    species: scientificName,
    date,
    hour: hour.toString(),
    duration: durationHours.toString(),
  });
  if (numResults !== undefined) params.set('numResults', numResults.toString());
  if (offset !== undefined) params.set('offset', offset.toString());
  return buildAppUrl(`/ui/detections?${params.toString()}`);
}
