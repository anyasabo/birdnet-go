import { describe, expect, it } from 'vitest';
import { createRawSnippet } from 'svelte';
import { screen } from '@testing-library/svelte';
import { renderTyped } from '../../../../test/render-helpers';
import SpeciesLink from './SpeciesLink.svelte';

const children = createRawSnippet(() => ({ render: () => '<span>Great Horned Owl</span>' }));

describe('SpeciesLink', () => {
  it('links to the species across all dates by default', () => {
    renderTyped(SpeciesLink, { props: { scientificName: 'Bubo virginianus', children } });

    const href = screen.getByRole('link', { name: 'Great Horned Owl' }).getAttribute('href') ?? '';
    const params = new URL(href, 'http://localhost').searchParams;
    expect(params.get('queryType')).toBe('species');
    expect(params.get('species')).toBe('Bubo virginianus');
    expect(params.has('date')).toBe(false);
    expect(params.has('start_date')).toBe(false);
  });

  it('carries the date range into the link', () => {
    renderTyped(SpeciesLink, {
      props: {
        scientificName: 'Bubo virginianus',
        range: { startDate: '2026-10-01', endDate: '2026-10-31' },
        children,
      },
    });

    const href = screen.getByRole('link').getAttribute('href') ?? '';
    const params = new URL(href, 'http://localhost').searchParams;
    expect(params.get('start_date')).toBe('2026-10-01');
    expect(params.get('end_date')).toBe('2026-10-31');
  });

  it('renders the content without a link when the species is unknown', () => {
    renderTyped(SpeciesLink, { props: { scientificName: '', className: 'kept', children } });

    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.getByText('Great Horned Owl').parentElement).toHaveClass('kept');
  });
});
