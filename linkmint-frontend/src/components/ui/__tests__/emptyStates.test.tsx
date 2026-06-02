/**
 * Empty-state catalog — branded copy renders per surface, title/description overrides win, the context
 * CTA renders, and each convenience wrapper shows its catalog title.
 */

import type { ReactElement } from 'react';
import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import {
  CatalogEmptyState,
  EMPTY_STATES,
  NoApiKeysEmpty,
  NoPayLinksEmpty,
  NoPaymentsEmpty,
  NoSearchResultsEmpty,
} from '@/components/ui';

const WRAPPERS: { name: string; node: () => ReactElement; title: string }[] = [
  { name: 'NoPaymentsEmpty', node: () => <NoPaymentsEmpty />, title: EMPTY_STATES.payments.title },
  {
    name: 'NoSearchResultsEmpty',
    node: () => <NoSearchResultsEmpty />,
    title: EMPTY_STATES.searchResults.title,
  },
  { name: 'NoApiKeysEmpty', node: () => <NoApiKeysEmpty />, title: EMPTY_STATES.apiKeys.title },
];

describe('empty-state catalog', () => {
  it('renders branded copy for a surface (heading + description)', () => {
    renderWithTheme(<CatalogEmptyState surface="paylinks" />);
    expect(screen.getByRole('heading', { name: EMPTY_STATES.paylinks.title })).toBeInTheDocument();
    expect(screen.getByText(EMPTY_STATES.paylinks.description)).toBeInTheDocument();
  });

  it('title/description overrides win', () => {
    renderWithTheme(
      <CatalogEmptyState
        surface="paylinks"
        title="No pending PayLinks"
        description="Custom copy."
      />,
    );
    expect(screen.getByRole('heading', { name: 'No pending PayLinks' })).toBeInTheDocument();
    expect(screen.getByText('Custom copy.')).toBeInTheDocument();
  });

  it('renders the context CTA', () => {
    renderWithTheme(<NoPayLinksEmpty action={<button>Create PayLink</button>} />);
    expect(screen.getByRole('button', { name: 'Create PayLink' })).toBeInTheDocument();
  });

  it.each(WRAPPERS)('$name renders its catalog title', ({ node, title }) => {
    renderWithTheme(node());
    expect(screen.getByRole('heading', { name: title })).toBeInTheDocument();
  });
});
