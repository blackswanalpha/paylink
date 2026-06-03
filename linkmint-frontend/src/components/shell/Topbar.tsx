'use client';

/**
 * Topbar — sticky header. On mobile it carries the brand (the sidebar is hidden) plus a compact
 * horizontal nav; on all sizes it shows the account cluster on the right. The account cluster reflects
 * the real identity session (work09): a user menu (Account / Sign out) when authed, a "Sign in" link
 * when anonymous. It bootstraps the session once so login state is correct on any page — including the
 * HS256 paylinks demo at /dashboard, which itself runs on a separate token (see lib/linkmint.ts).
 */

import { useEffect } from 'react';
import NextLink from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { Box, Button, Flex, HStack, Text } from '@chakra-ui/react';
import { ChevronDown, LogOut, User } from 'react-feather';

import { NotificationBell } from '@/components/notifications/NotificationBell';
import { Menu } from '@/components/ui';
import { bootstrapSession, logout } from '@/lib/authClient';
import { notify } from '@/lib/notify';
import { useAuthStore } from '@/store/auth';
import { NAV_ITEMS } from './nav';

/** Initial for the avatar chip — first letter of the email local-part, fallback "U". */
function initialFor(email: string | null | undefined): string {
  return (email?.trim()?.[0] ?? 'U').toUpperCase();
}

function AccountCluster() {
  const status = useAuthStore((s) => s.status);
  const user = useAuthStore((s) => s.user);
  const router = useRouter();

  useEffect(() => {
    if (status === 'unknown') {
      void bootstrapSession();
    }
  }, [status]);

  if (status === 'unknown') {
    return null; // resolving — avoid a sign-in/menu flash
  }

  if (status !== 'authed') {
    return (
      <Button asChild variant="outline" size="sm">
        <NextLink href="/login">Sign in</NextLink>
      </Button>
    );
  }

  const display = user?.email ?? user?.user_id ?? 'Account';

  return (
    <Menu
      placement="bottom-end"
      trigger={
        <Button variant="ghost" size="sm" gap={2} px={2} aria-label="Account menu">
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
            {initialFor(user?.email)}
          </Box>
          <Text
            display={{ base: 'none', sm: 'block' }}
            fontSize="sm"
            fontWeight="500"
            maxW="180px"
            truncate
          >
            {display}
          </Text>
          <ChevronDown size={16} />
        </Button>
      }
      items={[
        { value: 'account', label: 'Account', icon: <User size={14} /> },
        { value: 'logout', label: 'Sign out', icon: <LogOut size={14} />, tone: 'danger' },
      ]}
      onSelect={(value) => {
        if (value === 'account') {
          router.push('/account');
        } else if (value === 'logout') {
          void logout().then(() => {
            notify.success('Signed out');
            router.push('/login');
          });
        }
      }}
    />
  );
}

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

      {/* Right cluster: notification bell + account menu */}
      <HStack gap={1}>
        <NotificationBell />
        <AccountCluster />
      </HStack>
    </Flex>
  );
}
