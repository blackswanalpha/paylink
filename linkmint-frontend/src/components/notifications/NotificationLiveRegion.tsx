'use client';

/**
 * NotificationLiveRegion — a visually-hidden polite `aria-live` region (work07 / F.6) that announces
 * a NEW top-of-inbox notification even when the panel is closed. It seeds silently on the first
 * populated state (so it never reads the whole inbox aloud on load) and announces only when a newer
 * notification arrives afterward. Mounted once, app-wide, in Provider.
 */

import { useEffect, useRef, useState } from 'react';
import { Box } from '@chakra-ui/react';
import { useNotificationStore } from '@/store/notifications';

export function NotificationLiveRegion() {
  const newest = useNotificationStore((s) => s.items[0]);
  const [message, setMessage] = useState('');
  const seeded = useRef(false);
  const lastId = useRef<string | null>(null);

  useEffect(() => {
    if (!newest) return;
    if (!seeded.current) {
      // First populated render (e.g. initial fetch): remember the top item, announce nothing.
      seeded.current = true;
      lastId.current = newest.id;
      return;
    }
    if (newest.id !== lastId.current) {
      lastId.current = newest.id;
      setMessage(`${newest.kind} notification: ${newest.title}`);
    }
  }, [newest]);

  return (
    <Box srOnly role="status" aria-live="polite" aria-atomic="true">
      {message}
    </Box>
  );
}
