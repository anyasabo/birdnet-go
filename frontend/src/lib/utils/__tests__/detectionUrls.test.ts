import type { DetectionsListData } from '$lib/types/detection.types';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  buildBatchResolveQuery,
  buildDetectionDetailUrl,
  buildHourlyDetectionUrl,
  buildSpeciesRangeDetectionUrl,
  parseDetectionDateScope,
  buildSpeciesDetectionUrl,
  buildSpeciesHourUrl,
} from '../detectionUrls';
import { resetBasePath, setBasePath } from '../urlHelpers';

// Shared test fixtures so a change to the date, basepath, or species value
// only needs to happen in one place.
const TEST_DATE = '2026-04-15';
const TEST_BASE_PATH = '/birdnet';
const TEST_SPECIES = 'Turdus merula';

describe('detectionUrls', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetBasePath();
  });

  describe('buildDetectionDetailUrl', () => {
    it('points at the detection page under the basepath', () => {
      setBasePath('/birdnet');
      expect(buildDetectionDetailUrl(42)).toBe('/birdnet/ui/detections/42');
    });
  });

  describe('buildSpeciesRangeDetectionUrl', () => {
    it('omits every date for an all-dates link', () => {
      expect(buildSpeciesRangeDetectionUrl('Bubo virginianus')).toBe(
        '/ui/detections?queryType=species&species=Bubo+virginianus'
      );
    });

    it('drops a range with one bound', () => {
      expect(buildSpeciesRangeDetectionUrl('Bubo virginianus', { startDate: '2026-10-01' })).toBe(
        '/ui/detections?queryType=species&species=Bubo+virginianus'
      );
    });

    it('adds the range bounds', () => {
      expect(
        buildSpeciesRangeDetectionUrl('Bubo virginianus', {
          startDate: '2026-10-01',
          endDate: '2026-10-31',
        })
      ).toBe(
        '/ui/detections?queryType=species&species=Bubo+virginianus&start_date=2026-10-01&end_date=2026-10-31'
      );
      expect(
        buildSpeciesRangeDetectionUrl('Bubo virginianus', { startDate: null, endDate: null })
      ).toBe('/ui/detections?queryType=species&species=Bubo+virginianus');
    });
  });

  describe('buildBatchResolveQuery', () => {
    const data: DetectionsListData = {
      notes: [],
      queryType: 'species',
      species: 'Bubo virginianus',
      date: '2026-11-01',
      numResults: 25,
      offset: 0,
      totalResults: 0,
      itemsPerPage: 25,
      currentPage: 1,
      totalPages: 1,
      showingFrom: 0,
      showingTo: 0,
    };

    it('sends the range of the list, not the display date', () => {
      const query = buildBatchResolveQuery({
        ...data,
        startDate: '2026-10-01',
        endDate: '2026-10-31',
      });
      expect(query).toMatchObject({ startDate: '2026-10-01', endDate: '2026-10-31' });
      expect(query.date).toBeUndefined();
    });

    it('sends no date for an all-dates list', () => {
      expect(buildBatchResolveQuery(data).date).toBeUndefined();
    });

    it('sends the date of a single-day list', () => {
      expect(buildBatchResolveQuery({ ...data, filterDate: '2026-10-05' }).date).toBe('2026-10-05');
    });
  });

  describe('parseDetectionDateScope', () => {
    const today = '2026-11-01';
    const scope = (search: string, queryType: string) =>
      parseDetectionDateScope(new URLSearchParams(search), queryType, today);

    it('keeps an explicit date', () => {
      expect(scope('date=2026-10-05', 'species')).toEqual({ date: '2026-10-05' });
    });

    it('lets a range replace the single date', () => {
      expect(scope('start_date=2026-10-01&end_date=2026-10-31', 'species')).toEqual({
        start_date: '2026-10-01',
        end_date: '2026-10-31',
      });
    });

    it('ignores a range with one bound, which the backend would not apply', () => {
      expect(scope('start_date=2026-10-01', 'species')).toEqual({ date: undefined });
      expect(scope('end_date=2026-10-31', 'all')).toEqual({ date: today });
    });

    it('spans all dates for species and search lists without a date', () => {
      expect(scope('', 'species').date).toBeUndefined();
      expect(scope('', 'search').date).toBeUndefined();
    });

    it('defaults other lists to today', () => {
      expect(scope('', 'all').date).toBe(today);
      expect(scope('', 'hourly').date).toBe(today);
    });
  });

  describe('buildHourlyDetectionUrl', () => {
    it('returns unprefixed URL when no basepath is set', () => {
      const url = buildHourlyDetectionUrl(TEST_DATE, 7, 1);
      expect(url).toBe(`/ui/detections?queryType=hourly&date=${TEST_DATE}&hour=7&duration=1`);
    });

    it(`prepends ${TEST_BASE_PATH} when setBasePath(${TEST_BASE_PATH}) is active`, () => {
      setBasePath(TEST_BASE_PATH);
      const url = buildHourlyDetectionUrl(TEST_DATE, 7, 1);
      expect(url).toBe(
        `${TEST_BASE_PATH}/ui/detections?queryType=hourly&date=${TEST_DATE}&hour=7&duration=1`
      );
    });

    it('prepends Home Assistant ingress basepath', () => {
      setBasePath('/api/hassio_ingress/TOKEN');
      const url = buildHourlyDetectionUrl(TEST_DATE, 7, 1);
      expect(url).toBe(
        `/api/hassio_ingress/TOKEN/ui/detections?queryType=hourly&date=${TEST_DATE}&hour=7&duration=1`
      );
    });

    it('is idempotent when called twice with same args', () => {
      setBasePath(TEST_BASE_PATH);
      const a = buildHourlyDetectionUrl(TEST_DATE, 7, 1);
      const b = buildHourlyDetectionUrl(TEST_DATE, 7, 1);
      expect(a).toBe(b);
    });

    it('includes numResults and offset when provided', () => {
      const url = buildHourlyDetectionUrl(TEST_DATE, 7, 1, 50, 0);
      expect(url).toBe(
        `/ui/detections?queryType=hourly&date=${TEST_DATE}&hour=7&duration=1&numResults=50&offset=0`
      );
    });
  });

  describe('buildSpeciesDetectionUrl', () => {
    it('returns unprefixed URL with URL-encoded species name', () => {
      const url = buildSpeciesDetectionUrl(TEST_SPECIES, TEST_DATE);
      expect(url).toBe(`/ui/detections?queryType=species&species=Turdus+merula&date=${TEST_DATE}`);
    });

    it('prepends basepath when set', () => {
      setBasePath(TEST_BASE_PATH);
      const url = buildSpeciesDetectionUrl(TEST_SPECIES, TEST_DATE);
      expect(url).toBe(
        `${TEST_BASE_PATH}/ui/detections?queryType=species&species=Turdus+merula&date=${TEST_DATE}`
      );
    });

    it('includes numResults and offset when provided', () => {
      const url = buildSpeciesDetectionUrl(TEST_SPECIES, TEST_DATE, 50, 10);
      expect(url).toBe(
        `/ui/detections?queryType=species&species=Turdus+merula&date=${TEST_DATE}&numResults=50&offset=10`
      );
    });
  });

  describe('buildSpeciesHourUrl', () => {
    it('returns unprefixed URL with hour and duration', () => {
      const url = buildSpeciesHourUrl(TEST_SPECIES, TEST_DATE, 7, 2);
      expect(url).toBe(
        `/ui/detections?queryType=species&species=Turdus+merula&date=${TEST_DATE}&hour=7&duration=2`
      );
    });

    it('prepends basepath when set', () => {
      setBasePath(TEST_BASE_PATH);
      const url = buildSpeciesHourUrl(TEST_SPECIES, TEST_DATE, 7, 2);
      expect(url).toBe(
        `${TEST_BASE_PATH}/ui/detections?queryType=species&species=Turdus+merula&date=${TEST_DATE}&hour=7&duration=2`
      );
    });

    it('encodes species names with non-ASCII characters', () => {
      const url = buildSpeciesHourUrl('Pöllö lajinimi', TEST_DATE, 7, 1);
      expect(url).toBe(
        `/ui/detections?queryType=species&species=P%C3%B6ll%C3%B6+lajinimi&date=${TEST_DATE}&hour=7&duration=1`
      );
    });
  });
});
