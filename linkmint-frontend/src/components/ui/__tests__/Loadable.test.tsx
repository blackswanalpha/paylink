/**
 * Loadable — the loading/empty/data sequencer. Asserts the precedence: initial skeleton; a refresh
 * keeps data with no skeleton flash; an error defers to the caller (renders nothing) instead of a fake
 * empty (F.5); a failed refresh keeps stale data; empty; and data — plus the skeleton fallback exposing
 * an aria-busy region (F.6).
 */

import type { ComponentProps } from 'react';
import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { Loadable, MetricGridSkeleton } from '@/components/ui';

const SKELETON = <span data-testid="skel">SKELETON</span>;
const EMPTY = <span data-testid="empty">EMPTY</span>;
const DATA = <span data-testid="data">DATA</span>;

function setup(props: Partial<ComponentProps<typeof Loadable>>) {
  return renderWithTheme(
    <Loadable loading={false} isEmpty={false} skeleton={SKELETON} empty={EMPTY} {...props}>
      {DATA}
    </Loadable>,
  );
}

describe('Loadable', () => {
  it('shows the skeleton on initial load (loading, no data)', () => {
    setup({ loading: true, isEmpty: true, hasData: false });
    expect(screen.getByTestId('skel')).toBeInTheDocument();
    expect(screen.queryByTestId('empty')).toBeNull();
    expect(screen.queryByTestId('data')).toBeNull();
  });

  it('keeps data (no skeleton) on refresh (loading, data present)', () => {
    setup({ loading: true, isEmpty: false, hasData: true });
    expect(screen.getByTestId('data')).toBeInTheDocument();
    expect(screen.queryByTestId('skel')).toBeNull();
  });

  it('renders nothing on an initial error — never a fake empty (F.5)', () => {
    setup({ error: new Error('boom'), loading: false, isEmpty: true, hasData: false });
    expect(screen.queryByTestId('empty')).toBeNull();
    expect(screen.queryByTestId('skel')).toBeNull();
    expect(screen.queryByTestId('data')).toBeNull();
  });

  it('keeps stale data on a failed refresh (error with data present)', () => {
    setup({ error: new Error('boom'), loading: false, isEmpty: false, hasData: true });
    expect(screen.getByTestId('data')).toBeInTheDocument();
  });

  it('shows the branded empty when empty and not loading', () => {
    setup({ loading: false, isEmpty: true, hasData: false });
    expect(screen.getByTestId('empty')).toBeInTheDocument();
    expect(screen.queryByTestId('data')).toBeNull();
  });

  it('shows data when present', () => {
    setup({ loading: false, isEmpty: false });
    expect(screen.getByTestId('data')).toBeInTheDocument();
  });

  it('a skeleton fallback exposes an aria-busy region with a hidden label (F.6)', () => {
    renderWithTheme(
      <Loadable
        loading
        isEmpty
        hasData={false}
        skeleton={<MetricGridSkeleton count={2} />}
        empty={EMPTY}
      >
        {DATA}
      </Loadable>,
    );
    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByText(/loading metrics/i)).toBeInTheDocument();
  });
});
