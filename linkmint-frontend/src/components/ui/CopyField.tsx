'use client';

/**
 * CopyField — a labelled copy-to-clipboard field (frontendfeature.md §2.5); the block form of
 * AddressChip for instructions/detail panes (a PayLink id, a paybill account, a share URL). Reuses
 * AddressChip's copy idiom exactly: navigator.clipboard + a Sonner toast + a Copy↔Check flash.
 *
 * a11y (F.6): the copy button is labelled "Copy {label}"; the icon swap (Copy → Check) is a visible,
 * non-color confirmation alongside the toast. Chakra v3 also ships a `Clipboard.*` primitive — we
 * match the existing hand-rolled idiom instead, for consistency with AddressChip/KeyValueRow.
 */

import { useState } from 'react';
import { HStack, IconButton, Text } from '@chakra-ui/react';
import { Check, Copy } from 'react-feather';
import { notify } from '@/lib/notify';
import { Pop } from '@/motion';

export interface CopyFieldProps {
  /** The full value copied to the clipboard (even when the display is truncated). */
  value: string;
  /** Label used in the toast and the copy button's aria-label ("Copy {label}"). */
  label: string;
  /** Render the value in JetBrains Mono (addresses, ids, hashes). @default false */
  mono?: boolean;
  /** Truncate the *displayed* value (head…tail); the full value is still copied. */
  truncate?: { head: number; tail: number };
  /** 'block' = full-width row; 'inline' = compact pill (like AddressChip). @default 'block' */
  variant?: 'inline' | 'block';
  /** @default 'md' */
  size?: 'sm' | 'md';
}

function truncateValue(value: string, head: number, tail: number): string {
  if (value.length <= head + tail + 1) {
    return value;
  }
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}

export function CopyField({
  value,
  label,
  mono = false,
  truncate,
  variant = 'block',
  size = 'md',
}: CopyFieldProps) {
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

  const display = truncate ? truncateValue(value, truncate.head, truncate.tail) : value;
  const iconPx = size === 'sm' ? 14 : 16;
  const CopyIcon = copied ? Check : Copy;

  if (variant === 'inline') {
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
        <Text as="span" fontFamily={mono ? 'mono' : 'body'} fontSize="xs" color="fg" truncate>
          {display}
        </Text>
        <IconButton aria-label={`Copy ${label}`} size="2xs" variant="ghost" onClick={copy}>
          <Pop key={copied ? 'on' : 'off'} active={copied}>
            <CopyIcon size={12} />
          </Pop>
        </IconButton>
      </HStack>
    );
  }

  return (
    <HStack
      justify="space-between"
      gap={3}
      px={3}
      py={2}
      bg="surfaceSubtle"
      borderWidth="1px"
      borderColor="border"
      borderRadius="md"
    >
      <Text
        fontFamily={mono ? 'mono' : 'body'}
        fontSize={size === 'sm' ? 'sm' : 'md'}
        color="fg"
        truncate
      >
        {display}
      </Text>
      <IconButton
        aria-label={`Copy ${label}`}
        size={size === 'sm' ? 'xs' : 'sm'}
        variant="ghost"
        onClick={copy}
        flexShrink={0}
      >
        <Pop key={copied ? 'on' : 'off'} active={copied}>
          <CopyIcon size={iconPx} />
        </Pop>
      </IconButton>
    </HStack>
  );
}
