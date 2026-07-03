/**
 * Detection URL builders for the dashboard.
 *
 * These functions wrap buildAppUrl so every anchor the dashboard emits carries
 * the current basepath (handles YAML webserver.basepath, X-Forwarded-Prefix, and
 * Home Assistant X-Ingress-Path). Extracted from DailySummaryCard.svelte which
 * previously built these URLs inline without the basepath wrap, causing
 * hourly-grid and species-row anchors to 404 through reverse proxies when the
 * DB had detection rows (Forgejo #446).
 */

import { buildAppUrl } from './urlHelpers';

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
 * Builds a detection-list URL filtered to all detections of a species across a
 * date range (inclusive). Unlike buildSpeciesDetectionUrl, which pins a single
 * day, this passes start_date/end_date so the detections page routes through the
 * advanced-search path and returns every detection of the species in the range.
 * Used by the analytics Species and Activity pages to jump from a species to its
 * detections without collapsing to a single day.
 */
export function buildSpeciesRangeDetectionUrl(
  scientificName: string,
  startDate: string,
  endDate: string,
  numResults?: number,
  offset?: number
): string {
  const params = new URLSearchParams({
    queryType: 'species',
    species: scientificName,
    start_date: startDate,
    end_date: endDate,
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
