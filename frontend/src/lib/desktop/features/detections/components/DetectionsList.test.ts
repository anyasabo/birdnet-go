import { afterEach, describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { Detection, DetectionsListData } from '$lib/types/detection.types';
import { fetchWithCSRF } from '$lib/utils/api';
import DetectionsList from './DetectionsList.svelte';

vi.mock('$lib/utils/api', async importOriginal => ({
  ...(await importOriginal<typeof import('$lib/utils/api')>()),
  fetchWithCSRF: vi.fn(),
}));

const emptyData: DetectionsListData = {
  notes: [],
  queryType: 'all',
  date: '2026-01-01',
  numResults: 25,
  offset: 0,
  totalResults: 0,
  itemsPerPage: 25,
  currentPage: 1,
  totalPages: 1,
  showingFrom: 0,
  showingTo: 0,
};

describe('DetectionsList species titles', () => {
  const species: DetectionsListData = {
    ...emptyData,
    queryType: 'species',
    species: 'Bubo virginianus',
  };
  const renderTitle = (data: DetectionsListData) =>
    render(DetectionsList, {
      props: {
        data,
        onPageChange: vi.fn(),
        onDetailsClick: vi.fn(),
        onRefresh: vi.fn(),
        onNumResultsChange: vi.fn(),
        onSortChange: vi.fn(),
      },
    });

  it('names the range of a range list', () => {
    renderTitle({ ...species, startDate: '2026-10-01', endDate: '2026-10-31' });
    expect(screen.getByText('detections.titles.speciesRange')).toBeInTheDocument();
  });

  it('says when a species list spans all dates', () => {
    renderTitle(species);
    expect(screen.getByText('detections.titles.speciesAllDates')).toBeInTheDocument();
  });

  it('keeps the single-day title when the list filters on a date', () => {
    renderTitle({ ...species, filterDate: '2026-01-01' });
    expect(screen.getByText('detections.titles.species')).toBeInTheDocument();
  });
});

describe('DetectionsList select all matching', () => {
  const note: Detection = {
    id: 1,
    date: '2026-10-05',
    time: '06:00:00',
    beginTime: '2026-10-05T06:00:00',
    endTime: '2026-10-05T06:00:03',
    speciesCode: 'grhowl',
    scientificName: 'Bubo virginianus',
    commonName: 'Great Horned Owl',
    confidence: 0.9,
    verified: 'unverified',
    locked: false,
  };
  const rangeList: DetectionsListData = {
    ...emptyData,
    notes: [note],
    queryType: 'species',
    species: 'Bubo virginianus',
    date: '2026-11-01',
    startDate: '2026-10-01',
    endDate: '2026-10-31',
    totalResults: 60,
    showingFrom: 1,
    showingTo: 1,
  };

  const props = (data: DetectionsListData) => ({
    data,
    onPageChange: vi.fn(),
    onDetailsClick: vi.fn(),
    onRefresh: vi.fn(),
    onNumResultsChange: vi.fn(),
    onSortChange: vi.fn(),
  });

  async function selectAllMatching() {
    await fireEvent.click(screen.getByRole('button', { name: 'detections.selection.select' }));
    await fireEvent.click(screen.getAllByRole('checkbox')[0]);
    await fireEvent.click(
      screen.getByRole('button', { name: 'detections.selection.selectAllMatching' })
    );
  }

  afterEach(() => {
    vi.mocked(fetchWithCSRF).mockReset();
  });

  it('resolves the date range on screen, not the display date', async () => {
    vi.mocked(fetchWithCSRF).mockResolvedValue({ ids: ['1'], count: 1, processed: 1, skipped: 0 });
    render(DetectionsList, { props: props(rangeList) });
    await selectAllMatching();

    await fireEvent.click(
      screen.getByRole('button', { name: 'dashboard.recentDetections.modals.lockDetection' })
    );
    await fireEvent.click(await screen.findByRole('button', { name: /confirm/i }));

    await waitFor(() => expect(fetchWithCSRF).toHaveBeenCalled());
    const [url, init] = vi.mocked(fetchWithCSRF).mock.calls[0] ?? [];
    expect(url).toBe('/api/v2/detections/batch/resolve');
    const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
    expect(body).toMatchObject({ startDate: '2026-10-01', endDate: '2026-10-31' });
    expect(body).not.toHaveProperty('date');
  });

  it('drops the selection when the query changes', async () => {
    const { rerender } = render(DetectionsList, { props: props(rangeList) });
    await selectAllMatching();
    expect(screen.getByText('detections.selection.allSelected')).toBeInTheDocument();

    await rerender(props({ ...rangeList, species: 'Turdus merula' }));

    expect(screen.queryByText('detections.selection.allSelected')).toBeNull();
  });
});

describe('DetectionsList accessibility', () => {
  it('names the results-per-page dropdown independently of the selected page size', () => {
    render(DetectionsList, {
      props: {
        data: emptyData,
        onPageChange: vi.fn(),
        onDetailsClick: vi.fn(),
        onRefresh: vi.fn(),
        onNumResultsChange: vi.fn(),
        onSortChange: vi.fn(),
      },
    });

    const trigger = screen.getByRole('combobox', { name: 'detections.aria.resultsPerPage' });
    expect(trigger).toHaveTextContent('25');
  });
});
