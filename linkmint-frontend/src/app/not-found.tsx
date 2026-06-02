/**
 * Branded 404 (work04) — Next renders this for unmatched routes and `notFound()` calls. It renders
 * inside the root layout, so the Chakra theme is available. Server component: static, no interactivity.
 */

import NextLink from 'next/link';
import { Box, Button } from '@chakra-ui/react';
import { Compass, Home } from 'react-feather';
import { EmptyState } from '@/components/ui/EmptyState';

export default function NotFound() {
  return (
    <Box minH="60vh" display="grid" placeItems="center" px={6} py={16}>
      <EmptyState
        icon={<Compass size={24} />}
        title="Page not found"
        description="The page you’re looking for doesn’t exist or may have moved."
        action={
          <Button asChild colorPalette="emerald" gap={2}>
            <NextLink href="/">
              <Home size={16} /> Go home
            </NextLink>
          </Button>
        }
      />
    </Box>
  );
}
