'use client';

/**
 * NotificationItem — one inbox row: a kind-coloured dot, the title (bold while unread), an optional
 * body + relative time, and a "NEW" tag for unread. The unread signal is never colour-only (weight +
 * dot + tag — F.6). Activating the row marks it read; if it carries an `href`, it also navigates.
 */

import NextLink from 'next/link';
import { Box, HStack, Stack, Text, chakra } from '@chakra-ui/react';
import type { AppNotification, NotificationKind } from '@/store/notifications';

const DOT_COLOR: Record<NotificationKind, string> = {
  success: 'status.success',
  info: 'accent.solid',
  warning: 'status.pending',
  error: 'status.danger',
};

function relativeTime(ms: number): string {
  if (!ms) return '';
  const diff = ms - Date.now();
  const abs = Math.abs(diff);
  const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });
  const MIN = 60_000;
  const HR = 3_600_000;
  const DAY = 86_400_000;
  if (abs < MIN) return 'just now';
  if (abs < HR) return rtf.format(Math.round(diff / MIN), 'minute');
  if (abs < DAY) return rtf.format(Math.round(diff / HR), 'hour');
  return rtf.format(Math.round(diff / DAY), 'day');
}

export interface NotificationItemProps {
  item: AppNotification;
  /** Called when the row is activated (click / Enter); marks it read. */
  onActivate: () => void;
}

export function NotificationItem({ item, onActivate }: NotificationItemProps) {
  const dot = DOT_COLOR[item.kind] ?? 'accent.solid';
  const when = relativeTime(item.createdAt);

  const body = (
    <HStack
      align="flex-start"
      gap={3}
      w="100%"
      px={3}
      py={3}
      borderRadius="md"
      transition="background var(--lm-dur-fast, 120ms)"
      _hover={{ bg: 'surfaceSubtle' }}
    >
      <Box
        mt="6px"
        w="8px"
        h="8px"
        borderRadius="full"
        bg={item.read ? 'border' : dot}
        flexShrink={0}
        aria-hidden
      />
      <Stack gap={0.5} flex="1" minW={0}>
        <Text fontSize="sm" fontWeight={item.read ? '500' : '700'} color="fg">
          {item.title}
        </Text>
        {item.body ? (
          <Text fontSize="xs" color="fg.muted">
            {item.body}
          </Text>
        ) : null}
        {when ? (
          <Text fontSize="2xs" color="fg.muted">
            {when}
          </Text>
        ) : null}
      </Stack>
      {item.read ? null : (
        <Text fontSize="2xs" fontWeight="700" color={dot} flexShrink={0} mt="2px">
          NEW
        </Text>
      )}
    </HStack>
  );

  if (item.href) {
    return (
      <NextLink href={item.href} onClick={onActivate} style={{ textDecoration: 'none' }}>
        {body}
      </NextLink>
    );
  }

  return (
    <chakra.button
      type="button"
      textAlign="left"
      w="100%"
      onClick={onActivate}
      _focusVisible={{ outline: '2px solid', outlineColor: 'accent.solid', outlineOffset: '-2px' }}
    >
      {body}
    </chakra.button>
  );
}
