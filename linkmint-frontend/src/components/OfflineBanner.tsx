'use client';

/**
 * OfflineBanner — an app-wide connectivity banner (work04). Listens for the browser `online`/
 * `offline` events and shows a fixed banner while offline, plus a brief "Back online" confirmation on
 * reconnect. Mounted once in Provider.
 *
 * a11y (F.6): `role="status"` + `aria-live="polite"` so the change is announced; the state is conveyed
 * by an icon (WifiOff / Wifi) + text, never by color alone.
 */

import { useEffect, useState } from 'react';
import { Box, HStack, Text } from '@chakra-ui/react';
import { Wifi, WifiOff } from 'react-feather';

const BACK_ONLINE_MS = 3000;

export function OfflineBanner() {
  const [online, setOnline] = useState(true);
  const [showBackOnline, setShowBackOnline] = useState(false);

  useEffect(() => {
    // Read the live value after mount (avoids any SSR/CSR hydration mismatch).
    setOnline(navigator.onLine);

    let timer: ReturnType<typeof setTimeout> | null = null;
    const goOnline = (): void => {
      setOnline(true);
      setShowBackOnline(true);
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => setShowBackOnline(false), BACK_ONLINE_MS);
    };
    const goOffline = (): void => {
      setOnline(false);
      setShowBackOnline(false);
    };

    window.addEventListener('online', goOnline);
    window.addEventListener('offline', goOffline);
    return () => {
      window.removeEventListener('online', goOnline);
      window.removeEventListener('offline', goOffline);
      if (timer) clearTimeout(timer);
    };
  }, []);

  if (online && !showBackOnline) {
    return null;
  }

  const offline = !online;
  return (
    <Box
      role="status"
      aria-live="polite"
      position="fixed"
      top={0}
      insetX={0}
      zIndex={1400}
      bg={offline ? 'status.danger' : 'status.success'}
      color="white"
      py={2}
      px={4}
      textAlign="center"
    >
      <HStack justify="center" gap={2}>
        {offline ? <WifiOff size={16} /> : <Wifi size={16} />}
        <Text fontSize="sm" fontWeight="medium">
          {offline
            ? "You're offline — some actions are paused until your connection returns."
            : 'Back online.'}
        </Text>
      </HStack>
    </Box>
  );
}
