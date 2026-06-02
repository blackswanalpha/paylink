'use client';

/**
 * Sidebar — the desktop (md+) navigation rail: brand, nav list (active = emerald-tinted), a primary
 * "Create PayLink" CTA, and a non-custodial trust note in the footer. PLANNED routes render with a
 * "Soon" tag and are non-navigating (F.7).
 */

import NextLink from 'next/link';
import { usePathname } from 'next/navigation';
import { Box, Button, Flex, HStack, Stack, Text } from '@chakra-ui/react';
import { PlusCircle, Shield } from 'react-feather';
import { NAV_ITEMS, type NavItem } from './nav';

function NavRow({ item, active }: { item: NavItem; active: boolean }) {
  const Icon = item.icon;
  const content = (
    <HStack
      gap={3}
      px={3}
      py={2.5}
      borderRadius="md"
      color={active ? 'accent.fg' : 'fg.muted'}
      bg={active ? 'accent.subtle' : 'transparent'}
      fontWeight={active ? '600' : '500'}
      fontSize="sm"
      cursor={item.live ? 'pointer' : 'default'}
      opacity={item.live ? 1 : 0.55}
      transition="background 160ms, color 160ms"
      _hover={item.live && !active ? { bg: 'surfaceSubtle', color: 'fg' } : {}}
      aria-current={active ? 'page' : undefined}
    >
      <Icon size={18} />
      <Text>{item.label}</Text>
      {!item.live ? (
        <Text
          ml="auto"
          fontSize="2xs"
          fontWeight="600"
          textTransform="uppercase"
          letterSpacing="0.06em"
          color="fg.muted"
          bg="surfaceSubtle"
          px={1.5}
          py={0.5}
          borderRadius="full"
        >
          Soon
        </Text>
      ) : null}
    </HStack>
  );

  if (!item.live) {
    return (
      <Box aria-disabled title="Coming soon">
        {content}
      </Box>
    );
  }
  return (
    <NextLink href={item.href} style={{ textDecoration: 'none' }}>
      {content}
    </NextLink>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  return (
    <Flex
      as="aside"
      direction="column"
      display={{ base: 'none', md: 'flex' }}
      w="260px"
      flexShrink={0}
      h="100dvh"
      position="sticky"
      top={0}
      bg="bg.panel"
      borderRightWidth="1px"
      borderColor="border"
      px={4}
      py={5}
    >
      {/* Brand */}
      <HStack gap={2} px={2} mb={6}>
        <Text fontFamily="heading" fontSize="xl" color="accent.solid" lineHeight="1">
          ◇
        </Text>
        <Text fontFamily="heading" fontWeight="600" fontSize="xl" letterSpacing="-0.01em">
          LinkMint
        </Text>
      </HStack>

      <Button asChild variant="solid" colorPalette="emerald" size="sm" w="full" mb={6}>
        <NextLink href="/">
          <HStack gap={2}>
            <PlusCircle size={16} />
            <Text>Create PayLink</Text>
          </HStack>
        </NextLink>
      </Button>

      <Stack as="nav" gap={1} aria-label="Dashboard">
        {NAV_ITEMS.map((item) => (
          <NavRow key={item.href} item={item} active={item.live && pathname === item.href} />
        ))}
      </Stack>

      <HStack mt="auto" gap={2} px={2} pt={6} color="fg.muted">
        <Shield size={14} />
        <Text fontSize="xs">Non-custodial · funds never touch LinkMint</Text>
      </HStack>
    </Flex>
  );
}
