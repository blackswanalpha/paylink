'use client';

/**
 * NotificationBell — the topbar entry to the notification center (work07). Self-contained: it reads
 * the SDK client from the app store, drives the server-backed inbox via `useNotifications`, owns the
 * panel's open state, and refreshes on mount (client-ready) and on open. The unread count is on the
 * bell's `aria-label` (the badge is `aria-hidden` so it isn't announced twice — F.6).
 */

import { useEffect, useState } from 'react';
import { Box } from '@chakra-ui/react';
import { Bell } from 'react-feather';
import { IconButton } from '@/components/ui';
import { useNotifications } from '@/hooks/useNotifications';
import { useAppStore } from '@/store/app';
import { selectUnreadCount, useNotificationStore } from '@/store/notifications';
import { NotificationCenter } from './NotificationCenter';

export function NotificationBell() {
  const client = useAppStore((s) => s.client);
  const unread = useNotificationStore(selectUnreadCount);
  const { refresh, markRead, markAllRead } = useNotifications(client);
  const [open, setOpen] = useState(false);

  // Fetch the server-backed inbox once the SDK client is available.
  useEffect(() => {
    refresh();
  }, [refresh]);

  const openCenter = (): void => {
    setOpen(true);
    refresh(); // freshen on open
  };

  const label = unread > 0 ? `Notifications, ${unread} unread` : 'Notifications';
  const badge = unread > 9 ? '9+' : String(unread);

  return (
    <>
      <Box position="relative" display="inline-flex">
        <IconButton aria-label={label} variant="ghost" size="sm" onClick={openCenter}>
          <Bell size={18} />
        </IconButton>
        {unread > 0 ? (
          <Box
            position="absolute"
            top="-2px"
            insetEnd="-2px"
            minW="16px"
            h="16px"
            px="1"
            bg="accent.solid"
            color="white"
            borderRadius="full"
            fontSize="10px"
            fontWeight="700"
            lineHeight="16px"
            textAlign="center"
            pointerEvents="none"
            aria-hidden
          >
            {badge}
          </Box>
        ) : null}
      </Box>
      <NotificationCenter
        open={open}
        onClose={() => setOpen(false)}
        onMarkRead={markRead}
        onMarkAllRead={markAllRead}
      />
    </>
  );
}
