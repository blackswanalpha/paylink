/**
 * Skeleton compositions — each is an aria-busy loading region with a visually-hidden label and an
 * aria-hidden shimmer subtree (F.6/F.7), and renders without crashing under the Ivory theme.
 */

import type { ReactElement } from 'react';
import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import {
  SkeletonRegion,
  MetricGridSkeleton,
  TableSkeleton,
  DetailPanelSkeleton,
  FormSkeleton,
  ListCardSkeleton,
} from '@/components/ui';

const CASES: { name: string; node: () => ReactElement; label: RegExp }[] = [
  {
    name: 'MetricGridSkeleton',
    node: () => <MetricGridSkeleton count={3} />,
    label: /loading metrics/i,
  },
  { name: 'TableSkeleton', node: () => <TableSkeleton rows={3} />, label: /loading table/i },
  {
    name: 'DetailPanelSkeleton',
    node: () => <DetailPanelSkeleton rows={3} />,
    label: /loading details/i,
  },
  { name: 'FormSkeleton', node: () => <FormSkeleton fields={3} />, label: /loading form/i },
  { name: 'ListCardSkeleton', node: () => <ListCardSkeleton items={3} />, label: /loading list/i },
];

describe('skeleton compositions', () => {
  it('SkeletonRegion exposes aria-busy + a hidden label and hides the shimmer subtree', () => {
    const { container } = renderWithTheme(
      <SkeletonRegion label="widgets">
        <div data-testid="shimmer">blocks</div>
      </SkeletonRegion>,
    );
    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByText(/loading widgets/i)).toBeInTheDocument();
    expect(container.querySelector('[aria-hidden="true"]')).not.toBeNull();
  });

  it.each(CASES)('$name renders an aria-busy region with its label', ({ node, label }) => {
    renderWithTheme(node());
    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('MetricGridSkeleton renders multiple metric shells for its count', () => {
    const { container } = renderWithTheme(<MetricGridSkeleton count={4} />);
    // The shimmer subtree holds the grid + its 4 metric-card shells (each several blocks).
    const blocks = container.querySelectorAll('[aria-hidden="true"] *');
    expect(blocks.length).toBeGreaterThanOrEqual(4);
  });
});
