'use client';

/**
 * AppShell — the dashboard frame: a sticky Sidebar (md+) beside a main column of Topbar + scrollable
 * content on the ivory canvas. The content region is width-constrained and generously guttered (§2.4).
 */

import type { ReactNode } from 'react';
import { Box, Flex } from '@chakra-ui/react';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <Flex minH="100dvh" bg="canvas">
      <Sidebar />
      <Flex direction="column" flex="1" minW={0}>
        <Topbar />
        <Box as="main" flex="1" px={{ base: 4, md: 8 }} py={{ base: 6, md: 10 }}>
          <Box maxW="1200px" mx="auto">
            {children}
          </Box>
        </Box>
      </Flex>
    </Flex>
  );
}
