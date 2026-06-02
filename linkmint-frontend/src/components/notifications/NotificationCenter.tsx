'use client';

/**
 * NotificationCenter — the inbox panel (work07). A right-edge Drawer (kit `Drawer` → focus-trap,
 * role="dialog", Esc-to-close, labelled close — F.6) listing the server-backed notifications with a
 * "Mark all as read" footer and a branded empty state. Read state is owned by the store; the bell
 * supplies the mark handlers (which persist via the SDK).
 */

import { Stack } from '@chakra-ui/react';
import { Bell } from 'react-feather';
import { Button, Drawer, EmptyState } from '@/components/ui';
import { selectUnreadCount, useNotificationStore } from '@/store/notifications';
import { NotificationItem } from './NotificationItem';

export interface NotificationCenterProps {
  open: boolean;
  onClose: () => void;
  onMarkRead: (id: string) => void;
  onMarkAllRead: () => void;
}

export function NotificationCenter({
  open,
  onClose,
  onMarkRead,
  onMarkAllRead,
}: NotificationCenterProps) {
  const items = useNotificationStore((s) => s.items);
  const unread = useNotificationStore(selectUnreadCount);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      placement="end"
      size="sm"
      title="Notifications"
      description={unread > 0 ? `${unread} unread` : 'You’re all caught up'}
      footer={
        <Button variant="ghost" size="sm" onClick={onMarkAllRead} disabled={unread === 0}>
          Mark all as read
        </Button>
      }
    >
      {items.length === 0 ? (
        <EmptyState
          icon={<Bell size={22} />}
          title="No notifications yet"
          description="Settlement alerts and PayLink activity will show up here."
        />
      ) : (
        <Stack gap={1}>
          {items.map((item) => (
            <NotificationItem key={item.id} item={item} onActivate={() => onMarkRead(item.id)} />
          ))}
        </Stack>
      )}
    </Drawer>
  );
}
