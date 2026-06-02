'use client';

/** EmptyState — icon + Fraunces title + muted copy + an optional primary action (§2.5). */

import type { ReactNode } from 'react';
import { Box, Heading, Stack, Text } from '@chakra-ui/react';

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <Stack align="center" textAlign="center" gap={4} py={12} px={6}>
      {icon ? (
        <Box
          display="inline-flex"
          alignItems="center"
          justifyContent="center"
          w="56px"
          h="56px"
          borderRadius="full"
          bg="accent.subtle"
          color="accent.fg"
        >
          {icon}
        </Box>
      ) : null}
      <Stack gap={1} maxW="sm">
        <Heading as="h3" fontFamily="heading" fontWeight="600" size="lg">
          {title}
        </Heading>
        {description ? <Text color="fg.muted">{description}</Text> : null}
      </Stack>
      {action ? <Box mt={1}>{action}</Box> : null}
    </Stack>
  );
}
