'use client';

/** A label/value row with an optional copy button — used in the instructions and status views. */

import { HStack, IconButton, Text } from '@chakra-ui/react';
import { Copy } from 'react-feather';

export interface KeyValueRowProps {
  label: string;
  value: string;
  mono?: boolean;
  onCopy?: () => void;
}

export function KeyValueRow({ label, value, mono, onCopy }: KeyValueRowProps) {
  return (
    <HStack justify="space-between" align="start" gap={3}>
      <Text color="fg.muted" fontSize="sm" flexShrink={0}>
        {label}
      </Text>
      <HStack gap={2} align="start" minW={0}>
        <Text
          fontFamily={mono ? 'mono' : undefined}
          wordBreak={mono ? 'break-all' : undefined}
          textAlign="right"
        >
          {value}
        </Text>
        {onCopy ? (
          <IconButton aria-label={`Copy ${label}`} size="xs" variant="ghost" onClick={onCopy}>
            <Copy size={14} />
          </IconButton>
        ) : null}
      </HStack>
    </HStack>
  );
}
