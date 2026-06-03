'use client';

/**
 * ProtectedRoute — gate a page on an authenticated identity session. While the session is resolving
 * (cold-load bootstrap) it shows a minimal skeleton; if anonymous, `useRequireAuth` redirects to
 * `/login` and this renders nothing. Wrap the whole page (outside `AppShell`) so the chrome doesn't
 * flash before a redirect.
 */

import type { ReactNode } from 'react';
import { Box } from '@chakra-ui/react';

import { FormSkeleton } from '@/components/ui';
import { useRequireAuth } from '@/hooks/useRequireAuth';

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { ready } = useRequireAuth();

  if (!ready) {
    return (
      <Box
        maxW="480px"
        mx="auto"
        py={16}
        px={6}
        aria-busy="true"
        aria-label="Checking your session"
      >
        <FormSkeleton />
      </Box>
    );
  }

  return <>{children}</>;
}
