'use client';

/**
 * Topbar — sticky header. On mobile it carries the brand (the sidebar is hidden) plus a compact
 * horizontal nav; on all sizes it shows an account chip on the right. Demo-only account (no real
 * identity session yet — see frontendfeature.md §3.2 / §4).
 */

import NextLink from 'next/link';
import { usePathname } from 'next/navigation';
import { Box, Flex, HStack, Text } from '@chakra-ui/react';
import { ChevronDown } from 'react-feather';
import { NotificationBell } from '@/components/notifications/NotificationBell';
import { NAV_ITEMS } from './nav';

export function Topbar() {
  const pathname = usePathname();
  return (
    <Flex
      as="header"
      position="sticky"
      top={0}
      zIndex={10}
      align="center"
      justify="space-between"
      gap={4}
      h="64px"
      px={{ base: 4, md: 8 }}
      bg="rgba(255,255,255,0.85)"
      backdropFilter="saturate(180%) blur(8px)"
      borderBottomWidth="1px"
      borderColor="border"
    >
      {/* Mobile brand (sidebar hidden < md) */}
      <HStack gap={2} display={{ base: 'flex', md: 'none' }}>
        <Text fontFamily="heading" fontSize="lg" color="accent.solid" lineHeight="1">
          ◇
        </Text>
        <Text fontFamily="heading" fontWeight="600" fontSize="lg">
          LinkMint
        </Text>
      </HStack>

      {/* Mobile horizontal nav */}
      <HStack
        gap={1}
        display={{ base: 'flex', md: 'none' }}
        overflowX="auto"
        flex="1"
        justify="flex-end"
      >
        {NAV_ITEMS.filter((i) => i.live).map((item) => (
          <NextLink key={item.href} href={item.href} style={{ textDecoration: 'none' }}>
            <Text
              fontSize="sm"
              fontWeight={pathname === item.href ? '600' : '500'}
              color={pathname === item.href ? 'accent.fg' : 'fg.muted'}
              px={2}
              py={1}
            >
              {item.label}
            </Text>
          </NextLink>
        ))}
      </HStack>

      {/* Desktop workspace label */}
      <Text display={{ base: 'none', md: 'block' }} fontSize="sm" color="fg.muted" fontWeight="500">
        Merchant workspace
      </Text>

      {/* Right cluster: notification bell + account chip */}
      <HStack gap={1}>
        <NotificationBell />
        <HStack
          gap={2}
          px={2}
          py={1}
          borderRadius="full"
          _hover={{ bg: 'surfaceSubtle' }}
          cursor="pointer"
          title="Account (demo)"
        >
          <Box
            w="28px"
            h="28px"
            borderRadius="full"
            bg="accent.solid"
            color="white"
            display="flex"
            alignItems="center"
            justifyContent="center"
            fontFamily="heading"
            fontSize="sm"
            fontWeight="600"
          >
            M
          </Box>
          <Text display={{ base: 'none', sm: 'block' }} fontSize="sm" fontWeight="500">
            Merchant
          </Text>
          <ChevronDown size={16} />
        </HStack>
      </HStack>
    </Flex>
  );
}
