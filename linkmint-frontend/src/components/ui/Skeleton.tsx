'use client';

/**
 * Skeleton — a shimmer placeholder block (uses the `lm-pulse` keyframe in globals.css, which is
 * disabled under prefers-reduced-motion). Plus a couple of composed skeletons for the dashboard.
 */

import { Box, type BoxProps, HStack, Stack } from '@chakra-ui/react';
import { Panel } from './Panel';

export function Skeleton(props: BoxProps) {
  return (
    <Box
      bg="surfaceSubtle"
      borderRadius="md"
      animation="lm-pulse 1.4s ease-in-out infinite"
      {...props}
    />
  );
}

/** A MetricCard-shaped skeleton. */
export function MetricCardSkeleton() {
  return (
    <Panel p={5}>
      <Stack gap={4}>
        <Skeleton h="14px" w="40%" />
        <Skeleton h="32px" w="70%" />
      </Stack>
    </Panel>
  );
}

/** A table-row-shaped skeleton. */
export function TableRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <Stack gap={3}>
      {Array.from({ length: rows }, (_, i) => (
        <HStack key={i} justify="space-between" px={1}>
          <Skeleton h="16px" w="28%" />
          <Skeleton h="16px" w="18%" />
          <Skeleton h="16px" w="14%" />
          <Skeleton h="20px" w="80px" borderRadius="full" />
        </HStack>
      ))}
    </Stack>
  );
}
