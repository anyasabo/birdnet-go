import { describe, it, expect } from 'vitest';
import { parseDetectionQueryParams } from './detectionQuery';

// Fixed "today" so date-defaulting assertions are deterministic regardless of
// when the suite runs.
const TODAY = '2026-07-03';
const baseOpts = { savedResultsPerPage: 25, savedSortBy: null, today: TODAY };

describe('parseDetectionQueryParams', () => {
  describe('start_date/end_date range (species jump from analytics)', () => {
    it('forwards start_date and end_date from the URL', () => {
      const p = parseDetectionQueryParams(
        '?queryType=species&species=Turdus+merula&start_date=2026-06-01&end_date=2026-06-30',
        baseOpts
      );
      expect(p.queryType).toBe('species');
      expect(p.species).toBe('Turdus merula');
      expect(p.start_date).toBe('2026-06-01');
      expect(p.end_date).toBe('2026-06-30');
    });

    it('does NOT default date to today when a range is present (the core fix)', () => {
      const p = parseDetectionQueryParams(
        '?queryType=species&species=Turdus+merula&start_date=2026-06-01&end_date=2026-06-30',
        baseOpts
      );
      // A stray date=today would collapse the backend to a single day and hide
      // detections on other days — the exact bug this feature fixes.
      expect(p.date).toBeUndefined();
    });

    it('treats a lone start_date (no end_date) as a range and still suppresses the date default', () => {
      const p = parseDetectionQueryParams(
        '?queryType=species&species=Parus+major&start_date=2026-06-01',
        baseOpts
      );
      expect(p.start_date).toBe('2026-06-01');
      expect(p.end_date).toBeUndefined();
      expect(p.date).toBeUndefined();
    });

    it('lets an explicit date win even when a range is present', () => {
      const p = parseDetectionQueryParams(
        '?queryType=species&species=Parus+major&date=2026-06-15&start_date=2026-06-01&end_date=2026-06-30',
        baseOpts
      );
      expect(p.date).toBe('2026-06-15');
    });

    it('trims whitespace-only range params to undefined', () => {
      const p = parseDetectionQueryParams(
        '?queryType=species&species=Parus+major&start_date=%20%20&end_date=%20',
        baseOpts
      );
      expect(p.start_date).toBeUndefined();
      expect(p.end_date).toBeUndefined();
      // No usable range, non-search type -> falls back to today.
      expect(p.date).toBe(TODAY);
    });
  });

  describe('date defaulting without a range (unchanged behavior)', () => {
    it('defaults a bare species query to today (single day)', () => {
      const p = parseDetectionQueryParams('?queryType=species&species=Turdus+merula', baseOpts);
      expect(p.date).toBe(TODAY);
      expect(p.start_date).toBeUndefined();
      expect(p.end_date).toBeUndefined();
    });

    it('does not default date for search queries', () => {
      const p = parseDetectionQueryParams('?search=robin', baseOpts);
      expect(p.queryType).toBe('search');
      expect(p.date).toBeUndefined();
    });

    it('defaults queryType to all and date to today for an empty query', () => {
      const p = parseDetectionQueryParams('', baseOpts);
      expect(p.queryType).toBe('all');
      expect(p.date).toBe(TODAY);
    });

    it('infers search queryType when only a search param is present', () => {
      const p = parseDetectionQueryParams('?search=blackbird', baseOpts);
      expect(p.queryType).toBe('search');
      expect(p.search).toBe('blackbird');
    });
  });

  describe('numResults / sortBy / offset', () => {
    it('uses savedResultsPerPage when numResults is missing or invalid', () => {
      expect(parseDetectionQueryParams('', { ...baseOpts, savedResultsPerPage: 50 }).numResults).toBe(
        50
      );
      expect(
        parseDetectionQueryParams('?numResults=999', { ...baseOpts, savedResultsPerPage: 50 })
          .numResults
      ).toBe(50);
    });

    it('accepts a valid numResults from the URL', () => {
      expect(parseDetectionQueryParams('?numResults=100', baseOpts).numResults).toBe(100);
    });

    it('reads a valid sortBy from the URL over the saved value', () => {
      const p = parseDetectionQueryParams('?sortBy=confidence_desc', {
        ...baseOpts,
        savedSortBy: 'date_asc',
      });
      expect(p.sortBy).toBe('confidence_desc');
    });

    it('falls back to the saved sortBy when the URL omits it', () => {
      const p = parseDetectionQueryParams('', { ...baseOpts, savedSortBy: 'species_asc' });
      expect(p.sortBy).toBe('species_asc');
    });

    it('ignores an invalid sortBy and leaves it undefined when nothing is saved', () => {
      const p = parseDetectionQueryParams('?sortBy=bogus', baseOpts);
      expect(p.sortBy).toBeUndefined();
    });

    it('parses offset, defaulting to 0', () => {
      expect(parseDetectionQueryParams('?offset=25', baseOpts).offset).toBe(25);
      expect(parseDetectionQueryParams('', baseOpts).offset).toBe(0);
    });
  });
});
