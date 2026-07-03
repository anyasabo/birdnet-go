import { describe, it, expect, vi } from 'vitest';

import { CHART_REGISTRY } from './charts';
import type { AnalyticsParams, ChartPropsContext } from './types';

// The Activity-page charts that show species labels must thread ctx.onSpeciesClick
// through mapProps so a label/legend click can navigate to that species' detections.
// acoustic-succession is covered in its own file; this covers the other two
// (species-ridgeline, time-of-day-species), which have no dedicated registry test.

function makeParams(): AnalyticsParams {
  return {
    range: 'month',
    start: '2026-03-01',
    end: '2026-03-31',
    species: ['Turdus merula'],
    source: '',
    startDate: new Date('2026-03-01T00:00:00'),
    endDate: new Date('2026-03-31T00:00:00'),
  };
}

function makeCtx(): ChartPropsContext {
  return {
    options: {},
    onParamsChange: vi.fn(),
    speciesNames: new Map<string, string>(),
    onSpeciesClick: vi.fn(),
  };
}

function chartDef(id: string) {
  const def = CHART_REGISTRY.find(c => c.id === id);
  if (!def) throw new Error(`${id} chart def is required`);
  return def;
}

describe('Activity chart onSpeciesClick wiring', () => {
  it('species-ridgeline mapProps forwards ctx.onSpeciesClick', () => {
    const def = chartDef('species-ridgeline');
    const ctx = makeCtx();
    const props = def.mapProps?.([], makeParams(), ctx) ?? {};
    expect(props.onSpeciesClick).toBe(ctx.onSpeciesClick);
  });

  it('time-of-day-species mapProps forwards ctx.onSpeciesClick', () => {
    const def = chartDef('time-of-day-species');
    const ctx = makeCtx();
    const props = def.mapProps?.([], makeParams(), ctx) ?? {};
    expect(props.onSpeciesClick).toBe(ctx.onSpeciesClick);
  });
});
