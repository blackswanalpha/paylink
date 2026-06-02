'use client';

/**
 * Empty-state catalog (work06 / frontendfeature.md §1, §2.5) — standardized branded copy + icon per
 * surface, built on `EmptyState`. The registry holds copy/icon descriptors; thin wrappers accept the
 * surface-specific CTA (`action`), since the action is context-owned (e.g. a page provides the
 * "Create PayLink" link).
 *
 * F.5: these are "no data yet" states only — an empty-because-the-fetch-errored must NOT render here;
 * that precedence is enforced upstream in `Loadable` (error wins over empty).
 */

import type { ReactNode } from 'react';
import { CreditCard, Inbox, Key, Search } from 'react-feather';
import { EmptyState } from './EmptyState';

export type EmptySurface = 'paylinks' | 'payments' | 'searchResults' | 'apiKeys';

interface EmptyDescriptor {
  icon: ReactNode;
  title: string;
  description: string;
}

/** Branded copy + icon per surface. Keep copy in the LinkMint voice (warm, concrete, no jargon). */
export const EMPTY_STATES: Record<EmptySurface, EmptyDescriptor> = {
  paylinks: {
    icon: <Inbox size={24} />,
    title: 'No PayLinks yet',
    description:
      'Create your first PayLink and share it — it will appear here as it moves to settled.',
  },
  payments: {
    icon: <CreditCard size={24} />,
    title: 'No payments yet',
    description: 'Payments against your PayLinks will show up here once the first one comes in.',
  },
  searchResults: {
    icon: <Search size={24} />,
    title: 'No matches',
    description: 'Nothing matched your search. Try a different term or clear the filters.',
  },
  apiKeys: {
    icon: <Key size={24} />,
    title: 'No API keys',
    description: 'Create an API key to start integrating LinkMint into your own product.',
  },
};

export interface CatalogEmptyStateProps {
  surface: EmptySurface;
  /** Override the catalog title (e.g. a filtered subset: "No pending PayLinks"). */
  title?: string;
  /** Override the catalog description. */
  description?: string;
  /** The surface-specific CTA (e.g. a "Create PayLink" button). */
  action?: ReactNode;
}

/** Catalog-driven empty state: looks up branded copy/icon for the surface, accepts the context CTA. */
export function CatalogEmptyState({ surface, title, description, action }: CatalogEmptyStateProps) {
  const d = EMPTY_STATES[surface];
  return (
    <EmptyState
      icon={d.icon}
      title={title ?? d.title}
      description={description ?? d.description}
      action={action}
    />
  );
}

/** Convenience: the PayLinks empty state. Pass the "Create PayLink" CTA. */
export function NoPayLinksEmpty({ action }: { action?: ReactNode }) {
  return <CatalogEmptyState surface="paylinks" action={action} />;
}

/** Convenience: the payments empty state. */
export function NoPaymentsEmpty({ action }: { action?: ReactNode }) {
  return <CatalogEmptyState surface="payments" action={action} />;
}

/** Convenience: the no-search-results empty state. */
export function NoSearchResultsEmpty({ action }: { action?: ReactNode }) {
  return <CatalogEmptyState surface="searchResults" action={action} />;
}

/** Convenience: the API-keys empty state. Pass the "Create key" CTA. */
export function NoApiKeysEmpty({ action }: { action?: ReactNode }) {
  return <CatalogEmptyState surface="apiKeys" action={action} />;
}
