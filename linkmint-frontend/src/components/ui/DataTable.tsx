'use client';

/**
 * DataTable — a generic, token-styled table over Chakra v3 `Table.*` (frontendfeature.md §2.5).
 * Columns are data (not JSX children), which enables client-side sort (with `aria-sort`), a cursor
 * "Load more", loading/empty states, and a row-card collapse seam for work20. It is presentational
 * over already-fetched rows: pagination is driven by the caller via `hasMore`/`onLoadMore` (the SDK
 * paginates with `next_cursor`), and sort applies to the rows currently in memory.
 *
 * Reference migration (NOT performed here): the dashboard's hand-rolled "Recent PayLinks" table maps to
 *   { key: 'pl', header: 'PayLink', render: (pl) => <AddressChip value={pl.pl_id} label="PayLink id" /> }
 *   { key: 'amount', header: 'Amount', align: 'end', sortable: true, sortValue: (pl) => pl.amount, render: … }
 *
 * a11y (F.6): sortable headers are real <button>s with a chevron (a non-color sort cue); the <th> sets
 * `aria-sort` only when sortable. A visually-hidden `<caption>` names the table. Status cells compose
 * <StatusPill> so status is never color-only. `onRowClick` is a pointer affordance — keep a
 * keyboard-reachable control inside a cell for keyboard users (the work20 card layout will formalize this).
 */

import { useMemo, useState, type ReactNode } from 'react';
import { Box, Button, Flex, Table } from '@chakra-ui/react';
import { motion } from 'framer-motion';
import { ChevronDown, ChevronUp } from 'react-feather';
import { Skeleton } from './Skeleton';
import { useReducedMotion } from '@/hooks/useReducedMotion';
import { DURATION, EASE, STAGGER_STEP } from '@/motion/tokens';

/** A motion-enabled table row for the entrance stagger (see the `staggerIn` prop). */
const MotionTableRow = motion.create(Table.Row);

export type ColumnAlign = 'start' | 'center' | 'end';
export type SortDirection = 'asc' | 'desc';

export interface DataTableColumn<T> {
  key: string;
  header: ReactNode;
  render: (row: T, rowIndex: number) => ReactNode;
  sortable?: boolean;
  /** Comparable value for client-side sort; required on sortable columns (sort on data, not nodes). */
  sortValue?: (row: T) => string | number | Date;
  /** Cell + header alignment. @default 'start' */
  align?: ColumnAlign;
  /** Fixed/min width (Chakra width token or CSS length). */
  width?: string;
  /** Hide below a breakpoint — seam for the work20 responsive card layout. */
  hideBelow?: 'sm' | 'md' | 'lg';
  /** work20 seam: label for this column's value when later rendered as a card field. */
  cardLabel?: ReactNode;
}

export interface DataTableSort {
  key: string;
  direction: SortDirection;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  /** @default 'md' */
  size?: 'sm' | 'md' | 'lg';
  /** Hover highlight on rows. @default false */
  interactive?: boolean;
  /**
   * Animate real data rows in with an entrance stagger on mount (work05). Off by default; skeleton
   * and empty states never animate (F.7). Reduced motion shows rows instantly (F.6).
   */
  staggerIn?: boolean;
  onRowClick?: (row: T) => void;
  /** Controlled sort. Omit (undefined) for internal sort state; `null` = controlled & unsorted. */
  sort?: DataTableSort | null;
  onSortChange?: (sort: DataTableSort | null) => void;
  defaultSort?: DataTableSort | null;
  /** Initial load — renders skeleton rows. @default false */
  loading?: boolean;
  /** @default 5 */
  skeletonRows?: number;
  /** Rendered when `rows` is empty and not loading (pass an <EmptyState/>). */
  empty?: ReactNode;
  /** Screen-reader caption naming the table. */
  caption?: ReactNode;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  /** @default 'Load more' */
  loadMoreLabel?: string;
}

function compare(a: string | number | Date, b: string | number | Date): number {
  if (typeof a === 'string' && typeof b === 'string') {
    return a.localeCompare(b);
  }
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

function nextSort(key: string, current: DataTableSort | null): DataTableSort | null {
  if (!current || current.key !== key) return { key, direction: 'asc' };
  if (current.direction === 'asc') return { key, direction: 'desc' };
  return null;
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  size = 'md',
  interactive = false,
  staggerIn = false,
  onRowClick,
  sort,
  onSortChange,
  defaultSort = null,
  loading = false,
  skeletonRows = 5,
  empty,
  caption,
  hasMore = false,
  loadingMore = false,
  onLoadMore,
  loadMoreLabel = 'Load more',
}: DataTableProps<T>) {
  const isControlled = sort !== undefined;
  const [internalSort, setInternalSort] = useState<DataTableSort | null>(defaultSort);
  const effectiveSort = isControlled ? sort : internalSort;

  const sortedRows = useMemo(() => {
    if (!effectiveSort) return rows;
    const col = columns.find((c) => c.key === effectiveSort.key);
    if (!col?.sortValue) return rows;
    const sortValue = col.sortValue;
    const dir = effectiveSort.direction === 'asc' ? 1 : -1;
    return [...rows].sort((a, b) => dir * compare(sortValue(a), sortValue(b)));
  }, [rows, columns, effectiveSort]);

  function handleSort(key: string) {
    const next = nextSort(key, effectiveSort ?? null);
    if (!isControlled) setInternalSort(next);
    onSortChange?.(next);
  }

  function ariaSort(col: DataTableColumn<T>): 'ascending' | 'descending' | 'none' | undefined {
    if (!col.sortable) return undefined;
    if (effectiveSort?.key === col.key) {
      return effectiveSort.direction === 'asc' ? 'ascending' : 'descending';
    }
    return 'none';
  }

  const colCount = columns.length;
  const reduced = useReducedMotion();

  return (
    <Box>
      <Box overflowX="auto">
        <Table.Root size={size} interactive={interactive}>
          {caption ? <Table.Caption srOnly>{caption}</Table.Caption> : null}
          <Table.Header>
            <Table.Row bg="surfaceSubtle">
              {columns.map((col) => {
                const active = effectiveSort?.key === col.key;
                const dir = active ? effectiveSort?.direction : undefined;
                return (
                  <Table.ColumnHeader
                    key={col.key}
                    color="fg.muted"
                    textAlign={col.align ?? 'start'}
                    width={col.width}
                    hideBelow={col.hideBelow}
                    aria-sort={ariaSort(col)}
                  >
                    {col.sortable ? (
                      <Button
                        type="button"
                        variant="plain"
                        size="xs"
                        onClick={() => handleSort(col.key)}
                        color="fg.muted"
                        fontWeight="600"
                        fontSize="xs"
                        letterSpacing="0.02em"
                        gap={1}
                        h="auto"
                        minH="0"
                        px="0"
                      >
                        {col.header}
                        <Box as="span" display="inline-flex" opacity={active ? 1 : 0.4}>
                          {dir === 'desc' ? <ChevronDown size={14} /> : <ChevronUp size={14} />}
                        </Box>
                      </Button>
                    ) : (
                      col.header
                    )}
                  </Table.ColumnHeader>
                );
              })}
            </Table.Row>
          </Table.Header>

          <Table.Body>
            {loading ? (
              Array.from({ length: skeletonRows }).map((_, r) => (
                <Table.Row key={`skeleton-${r}`}>
                  {columns.map((col) => (
                    <Table.Cell key={col.key}>
                      <Skeleton height="1rem" />
                    </Table.Cell>
                  ))}
                </Table.Row>
              ))
            ) : sortedRows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={colCount}>{empty}</Table.Cell>
              </Table.Row>
            ) : (
              sortedRows.map((row, rowIndex) => {
                const cells = columns.map((col) => (
                  <Table.Cell key={col.key} textAlign={col.align ?? 'start'}>
                    {col.render(row, rowIndex)}
                  </Table.Cell>
                ));
                const handlers = {
                  onClick: onRowClick ? () => onRowClick(row) : undefined,
                  cursor: onRowClick ? ('pointer' as const) : undefined,
                };
                return staggerIn ? (
                  <MotionTableRow
                    key={rowKey(row)}
                    initial={reduced ? false : { opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{
                      duration: DURATION.base,
                      ease: EASE,
                      delay: reduced ? 0 : rowIndex * STAGGER_STEP,
                    }}
                    {...handlers}
                  >
                    {cells}
                  </MotionTableRow>
                ) : (
                  <Table.Row key={rowKey(row)} {...handlers}>
                    {cells}
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table.Root>
      </Box>

      {hasMore ? (
        <Flex justify="center" py={4}>
          <Button
            variant="outline"
            size="sm"
            onClick={onLoadMore}
            loading={loadingMore}
            loadingText="Loading…"
          >
            {loadMoreLabel}
          </Button>
        </Flex>
      ) : null}
    </Box>
  );
}
