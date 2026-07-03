import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { select, type Selection } from 'd3-selection';
import { createLegend } from './interactions';

// createLegend appends to an SVG <g>. We give it a real (jsdom) SVG container so
// the d3 click/keydown listeners are exercised through actual DOM events, which is
// the only way to verify the label's stopPropagation vs the group's toggle.
const SVG_NS = 'http://www.w3.org/2000/svg';

let svg: SVGSVGElement;
let container: Selection<SVGGElement, unknown, null, undefined>;

// Fresh copies per call: createLegend mutates each item's `visible` in place on
// toggle, so a shared array would leak state between tests.
function makeItems() {
  return [
    { id: 'Turdus merula', label: 'Eurasian Blackbird', color: '#111', visible: true },
    { id: 'Parus major', label: 'Great Tit', color: '#222', visible: true },
  ];
}

beforeEach(() => {
  svg = document.createElementNS(SVG_NS, 'svg');
  document.body.appendChild(svg);
  const g = document.createElementNS(SVG_NS, 'g');
  svg.appendChild(g);
  container = select(g as SVGGElement);
});

afterEach(() => {
  svg.remove();
  vi.restoreAllMocks();
});

function labels() {
  return Array.from(svg.querySelectorAll<SVGTextElement>('.legend-item text'));
}
function rects() {
  return Array.from(svg.querySelectorAll<SVGRectElement>('.legend-item rect'));
}

describe('createLegend', () => {
  it('renders a swatch and label per item', () => {
    createLegend(container, { items: makeItems(), position: { x: 0, y: 0 }, itemHeight: 20 });
    expect(rects()).toHaveLength(2);
    expect(labels().map(t => t.textContent)).toEqual(['Eurasian Blackbird', 'Great Tit']);
  });

  it('toggles visibility on legend-item click and reports the item id', () => {
    const onToggle = vi.fn();
    createLegend(container, {
      items: makeItems(),
      position: { x: 0, y: 0 },
      itemHeight: 20,
      onToggle,
    });

    // Click the group (default toggle behavior; no onLabelClick configured).
    const group = svg.querySelector<SVGGElement>('.legend-item');
    expect(group).not.toBeNull();
    group?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    expect(onToggle).toHaveBeenCalledWith('Turdus merula', false);
  });

  describe('with onLabelClick', () => {
    it('marks labels as accessible links and navigates on label click', () => {
      const onLabelClick = vi.fn();
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onLabelClick,
        ariaLabel: id => `View detections for ${id}`,
      });

      const label = labels()[0];
      expect(label.getAttribute('role')).toBe('link');
      expect(label.getAttribute('tabindex')).toBe('0');
      expect(label.getAttribute('aria-label')).toBe('View detections for Turdus merula');

      label.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      expect(onLabelClick).toHaveBeenCalledWith('Turdus merula');
    });

    it('does NOT toggle visibility when the label is clicked (stopPropagation)', () => {
      const onToggle = vi.fn();
      const onLabelClick = vi.fn();
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onToggle,
        onLabelClick,
      });

      // A click on the label bubbles toward the group's toggle handler; the label
      // handler must stopPropagation so only navigation fires.
      labels()[0].dispatchEvent(new MouseEvent('click', { bubbles: true }));

      expect(onLabelClick).toHaveBeenCalledWith('Turdus merula');
      expect(onToggle).not.toHaveBeenCalled();
    });

    it('still toggles visibility when the swatch is clicked', () => {
      const onToggle = vi.fn();
      const onLabelClick = vi.fn();
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onToggle,
        onLabelClick,
      });

      // The swatch has no label handler, so its click bubbles to the group toggle.
      rects()[0].dispatchEvent(new MouseEvent('click', { bubbles: true }));

      expect(onToggle).toHaveBeenCalledWith('Turdus merula', false);
      expect(onLabelClick).not.toHaveBeenCalled();
    });

    it('navigates on Enter and Space keydown', () => {
      const onLabelClick = vi.fn();
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onLabelClick,
      });

      labels()[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
      labels()[1].dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));

      expect(onLabelClick).toHaveBeenCalledTimes(2);
      expect(onLabelClick).toHaveBeenNthCalledWith(1, 'Parus major');
      expect(onLabelClick).toHaveBeenNthCalledWith(2, 'Parus major');
    });

    it('ignores other keys', () => {
      const onLabelClick = vi.fn();
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onLabelClick,
      });

      labels()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true }));
      expect(onLabelClick).not.toHaveBeenCalled();
    });

    it('falls back to the label text for aria when no ariaLabel builder is given', () => {
      createLegend(container, {
        items: makeItems(),
        position: { x: 0, y: 0 },
        itemHeight: 20,
        onLabelClick: vi.fn(),
      });
      expect(labels()[0].getAttribute('aria-label')).toBe('Eurasian Blackbird');
    });
  });
});
