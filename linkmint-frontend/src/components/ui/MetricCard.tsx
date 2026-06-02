'use client';

/** MetricCard — a headline number (Fraunces) with label, optional delta chip and Sparkline (§2.5). */

import type { ReactNode } from 'react';
import { Box, Flex, HStack, Stack, Text } from '@chakra-ui/react';
import { Panel } from './Panel';
import { Sparkline } from './Sparkline';

export interface MetricCardProps {
  label: string;
  /** The headline value — a string or a composed node (e.g. <AmountDisplay/>). */
  value: ReactNode;
  /** Optional small leading icon. */
  icon?: ReactNode;
  /** Optional delta, e.g. "+12%". `positive` controls the tone. */
  delta?: { label: string; positive?: boolean };
  /** Optional sparkline series. */
  sparkline?: number[];
}

export function MetricCard({ label, value, icon, delta, sparkline }: MetricCardProps) {
  return (
    <Panel p={5}>
      <Stack gap={3}>
        <HStack justify="space-between" align="center">
          <HStack gap={2} color="fg.muted">
            {icon ? <Box display="inline-flex">{icon}</Box> : null}
            <Text fontSize="sm" fontWeight="500">
              {label}
            </Text>
          </HStack>
          {delta ? (
            <Text
              fontSize="xs"
              fontWeight="600"
              color={delta.positive === false ? 'status.danger' : 'status.success'}
            >
              {delta.label}
            </Text>
          ) : null}
        </HStack>

        <Flex justify="space-between" align="flex-end" gap={3}>
          <Text as="div" fontFamily="heading" fontWeight="600" fontSize="3xl" lineHeight="1.1">
            {value}
          </Text>
          {sparkline && sparkline.length > 0 ? (
            <Sparkline data={sparkline} width={120} height={40} />
          ) : null}
        </Flex>
      </Stack>
    </Panel>
  );
}
