'use client';

/**
 * Skeleton compositions per layout (work06 / frontendfeature.md §1). Built on the `Skeleton` primitive
 * (Skeleton.tsx). Each composition is a self-contained loading REGION: `aria-busy="true"` + a
 * visually-hidden "Loading <label>…" message, with the decorative shimmer marked `aria-hidden` so
 * screen readers never read placeholder blocks as data (F.6 + F.7).
 *
 * Skeletons NEVER use entrance/stagger motion — the only motion is the reduced-motion-gated `lm-pulse`
 * opacity keyframe already defined in globals.css (F.7). The metric-grid and table cases wrap the
 * existing `MetricCardSkeleton` / `TableRowsSkeleton`; detail/form/list are new shapes.
 */

import type { ReactNode } from 'react';
import { Box, HStack, SimpleGrid, Stack } from '@chakra-ui/react';
import { Panel } from './Panel';
import { Skeleton, MetricCardSkeleton, TableRowsSkeleton } from './Skeleton';

export interface SkeletonRegionProps {
  /** Visually-hidden announcement, e.g. "metrics", "PayLinks table". @default 'content' */
  label?: string;
  children: ReactNode;
}

/**
 * Wraps any skeleton composition as an `aria-busy` loading region with a visually-hidden label, and
 * hides the decorative shimmer subtree from assistive tech (F.6/F.7). Compose your own one-off
 * skeletons by passing `Skeleton` blocks as children.
 */
export function SkeletonRegion({ label = 'content', children }: SkeletonRegionProps) {
  return (
    <Box role="status" aria-busy="true">
      <Box as="span" srOnly>{`Loading ${label}…`}</Box>
      <Box aria-hidden>{children}</Box>
    </Box>
  );
}

/** N MetricCard-shaped skeletons in the dashboard's responsive grid. @default count 4 */
export function MetricGridSkeleton({
  count = 4,
  label = 'metrics',
}: {
  count?: number;
  label?: string;
}) {
  return (
    <SkeletonRegion label={label}>
      <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={5}>
        {Array.from({ length: count }, (_, i) => (
          <MetricCardSkeleton key={i} />
        ))}
      </SimpleGrid>
    </SkeletonRegion>
  );
}

/** Table-shaped skeleton region (wraps the existing `TableRowsSkeleton`). @default rows 5 */
export function TableSkeleton({ rows = 5, label = 'table' }: { rows?: number; label?: string }) {
  return (
    <SkeletonRegion label={label}>
      <Stack p={6}>
        <TableRowsSkeleton rows={rows} />
      </Stack>
    </SkeletonRegion>
  );
}

/** Detail-panel skeleton: a title + a stack of label/value rows (PayLink / payment detail). */
export function DetailPanelSkeleton({
  rows = 5,
  label = 'details',
}: {
  rows?: number;
  label?: string;
}) {
  return (
    <SkeletonRegion label={label}>
      <Panel p={6}>
        <Stack gap={5}>
          <Skeleton h="24px" w="50%" />
          <Stack gap={4}>
            {Array.from({ length: rows }, (_, i) => (
              <HStack key={i} justify="space-between">
                <Skeleton h="14px" w="30%" />
                <Skeleton h="14px" w="45%" />
              </HStack>
            ))}
          </Stack>
        </Stack>
      </Panel>
    </SkeletonRegion>
  );
}

/** Form skeleton: N field rows (label + control) + an action block. @default fields 3 */
export function FormSkeleton({ fields = 3, label = 'form' }: { fields?: number; label?: string }) {
  return (
    <SkeletonRegion label={label}>
      <Stack gap={5} maxW="sm">
        {Array.from({ length: fields }, (_, i) => (
          <Stack key={i} gap={2}>
            <Skeleton h="14px" w="25%" />
            <Skeleton h="40px" w="100%" borderRadius="md" />
          </Stack>
        ))}
        <Skeleton h="40px" w="120px" borderRadius="md" />
      </Stack>
    </SkeletonRegion>
  );
}

/** List-card skeleton: N stacked card rows (avatar + two text lines + trailing pill). @default items 4 */
export function ListCardSkeleton({
  items = 4,
  label = 'list',
}: {
  items?: number;
  label?: string;
}) {
  return (
    <SkeletonRegion label={label}>
      <Stack gap={3}>
        {Array.from({ length: items }, (_, i) => (
          <Panel key={i} p={4}>
            <HStack gap={4}>
              <Skeleton h="40px" w="40px" borderRadius="full" />
              <Stack gap={2} flex="1">
                <Skeleton h="16px" w="40%" />
                <Skeleton h="12px" w="60%" />
              </Stack>
              <Skeleton h="20px" w="72px" borderRadius="full" />
            </HStack>
          </Panel>
        ))}
      </Stack>
    </SkeletonRegion>
  );
}
