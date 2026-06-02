'use client';

/** PageHeader — Fraunces title + muted subtitle + an optional actions row (§2.5). */

import type { ReactNode } from 'react';
import { Flex, Heading, Stack, Text } from '@chakra-ui/react';

export interface PageHeaderProps {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}

export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <Flex
      direction={{ base: 'column', md: 'row' }}
      justify="space-between"
      align={{ base: 'flex-start', md: 'flex-end' }}
      gap={4}
    >
      <Stack gap={1}>
        <Heading as="h1" fontFamily="heading" fontWeight="600" size="2xl" letterSpacing="-0.01em">
          {title}
        </Heading>
        {subtitle ? (
          <Text color="fg.muted" fontSize="md">
            {subtitle}
          </Text>
        ) : null}
      </Stack>
      {actions ? <Flex gap={3}>{actions}</Flex> : null}
    </Flex>
  );
}
