'use client';

/**
 * AddressChip — a truncated mono identifier (0x address, PayLink id, tx hash) with a copy button.
 * Shows the head + tail of long hex values; copies the full value. Reuses the copy affordance idiom
 * from KeyValueRow.
 */

import { HStack, IconButton, Text } from '@chakra-ui/react';
import { Check, Copy } from 'react-feather';
import { useState } from 'react';
import { notify } from '@/lib/notify';
import { Pop } from '@/motion';

export interface AddressChipProps {
  value: string;
  /** Characters to keep at the head (after any 0x) and tail when truncating. */
  head?: number;
  tail?: number;
  /** Accessible label for the copy button, e.g. "Copy PayLink id". */
  label?: string;
}

function truncate(value: string, head: number, tail: number): string {
  if (value.length <= head + tail + 1) {
    return value;
  }
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}

export function AddressChip({ value, head = 6, tail = 4, label = 'value' }: AddressChipProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      notify.success(`Copied ${label}`);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      notify.error('Could not copy to clipboard');
    }
  }

  return (
    <HStack
      as="span"
      display="inline-flex"
      gap={1.5}
      px={2}
      py={1}
      bg="surfaceSubtle"
      borderRadius="md"
      maxW="100%"
    >
      <Text as="span" fontFamily="mono" fontSize="xs" color="fg" truncate>
        {truncate(value, head, tail)}
      </Text>
      <IconButton aria-label={`Copy ${label}`} size="2xs" variant="ghost" onClick={copy}>
        <Pop key={copied ? 'on' : 'off'} active={copied}>
          {copied ? <Check size={12} /> : <Copy size={12} />}
        </Pop>
      </IconButton>
    </HStack>
  );
}
