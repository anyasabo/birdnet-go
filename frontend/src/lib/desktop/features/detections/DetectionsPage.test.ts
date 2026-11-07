import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/svelte';
import { navigation } from '$lib/stores/navigation.svelte';
import { fetchWithCSRF } from '$lib/utils/api';
import type { DetectionsListData } from '$lib/types/detection.types';
import DetectionsCard from './components/DetectionsCard.svelte';
import DetectionsPage from './DetectionsPage.svelte';

vi.mock('$lib/utils/api', () => ({ fetchWithCSRF: vi.fn() }));
vi.mock('./components/DetectionsCard.svelte');

const SPECIES = 'queryType=species&species=Bubo+virginianus';

/** The query string of each list request so far. */
function requests(): URLSearchParams[] {
  return vi
    .mocked(fetchWithCSRF)
    .mock.calls.map(([url]) => new URLSearchParams(String(url).split('?')[1]));
}

interface CardProps {
  data: DetectionsListData | null;
  onPageChange: (_page: number) => void;
}

function cardProps(): CardProps {
  const call = vi.mocked(DetectionsCard).mock.calls.at(-1) as unknown[] | undefined;
  return call?.[1] as CardProps;
}

async function renderAt(search: string) {
  navigation.redirect(`/ui/detections?${search}`);
  render(DetectionsPage);
  await waitFor(() => expect(cardProps().data).not.toBeNull());
}

describe('DetectionsPage query scope', () => {
  const originalLocation = window.location;

  beforeEach(() => {
    // Shared test setup stubs window.location; use JSDOM's history-aware location.
    Object.defineProperty(window, 'location', { value: document.location, configurable: true });
    vi.mocked(fetchWithCSRF).mockResolvedValue({ data: [], total: 60, limit: 25 });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    Object.defineProperty(window, 'location', { value: originalLocation, configurable: true });
  });

  it('spans all dates for a species link without dates', async () => {
    await renderAt(SPECIES);

    expect(requests()[0]?.has('date')).toBe(false);
    expect(cardProps().data?.filterDate).toBeUndefined();
  });

  it('keeps the date of a single-day list', async () => {
    await renderAt(`${SPECIES}&date=2026-10-05`);

    expect(requests()[0]?.get('date')).toBe('2026-10-05');
    expect(cardProps().data?.filterDate).toBe('2026-10-05');
  });

  it('passes a range to the request and the list', async () => {
    await renderAt(`${SPECIES}&start_date=2026-10-01&end_date=2026-10-31`);

    expect(requests()[0]?.get('start_date')).toBe('2026-10-01');
    expect(requests()[0]?.get('end_date')).toBe('2026-10-31');
    expect(requests()[0]?.has('date')).toBe(false);
    expect(cardProps().data).toMatchObject({ startDate: '2026-10-01', endDate: '2026-10-31' });
  });

  it('refetches when a link changes the query on the same page', async () => {
    await renderAt(SPECIES);

    navigation.navigate('/ui/detections?queryType=species&species=Turdus+merula');

    await waitFor(() => expect(requests().at(-1)?.get('species')).toBe('Turdus merula'));
    expect(requests()).toHaveLength(2);
  });

  it('refetches a link back to the original query after paging', async () => {
    await renderAt(SPECIES);
    cardProps().onPageChange(2);
    await waitFor(() => expect(requests().at(-1)?.get('offset')).toBe('25'));

    navigation.navigate(`/ui/detections?${SPECIES}`);

    await waitFor(() => expect(requests().at(-1)?.get('offset')).toBe('0'));
  });

  it('does not fetch for the query of the page being navigated to', async () => {
    await renderAt(SPECIES);

    navigation.navigate('/ui/detections/42?tab=review');
    await Promise.resolve();

    expect(requests()).toHaveLength(1);
  });
});
