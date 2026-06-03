'use client';

/**
 * AccountScreen — the /account surface (work10). A tabbed self-serve area over the identity SDK:
 * Profile, Security (sessions + MFA), API keys, Organizations, and a PLANNED Notifications tab. Builds
 * one authed identity client (RS256 session, auto-refreshing) and hands it to each tab's data hook.
 */

import { useEffect, useMemo } from 'react';
import { Stack } from '@chakra-ui/react';
import { Bell, Key, Shield, User, Users } from 'react-feather';

import { createAuthedLinkMintClient, createLinkMintClient } from '@/lib/linkmint';
import { useAppStore } from '@/store/app';
import { PageHeader, Tabs, type TabItem } from '@/components/ui';
import { ApiKeysTab } from './ApiKeysTab';
import { NotificationsTab } from './NotificationsTab';
import { OrganizationsTab } from './OrganizationsTab';
import { ProfileTab } from './ProfileTab';
import { SecurityTab } from './SecurityTab';

export function AccountScreen({ notificationsToken }: { notificationsToken: string }) {
  // Identity (RS256) client for Profile/Security/API-keys/Organizations — the token provider reads
  // the live store on each request (auto-refresh). Stable across renders.
  const client = useMemo(() => createAuthedLinkMintClient(), []);

  // Separate HS256/creator-addr client for notification-service: its inbox + preferences are scoped
  // by the gateway-injected X-Creator-Addr, not the identity user_id. The two are never mixed.
  const notificationsClient = useMemo(
    () => createLinkMintClient(notificationsToken),
    [notificationsToken],
  );

  // Expose it globally so the Topbar notification bell drives the inbox on /account too (mirrors
  // MerchantDashboard; the dashboard replaces it with its own equivalent client when you go there).
  const setClient = useAppStore((s) => s.setClient);
  useEffect(() => {
    setClient(notificationsClient);
  }, [notificationsClient, setClient]);

  const items: TabItem[] = [
    {
      value: 'profile',
      label: 'Profile',
      icon: <User size={15} />,
      content: <ProfileTab client={client} />,
    },
    {
      value: 'security',
      label: 'Security',
      icon: <Shield size={15} />,
      content: <SecurityTab client={client} />,
    },
    {
      value: 'api-keys',
      label: 'API keys',
      icon: <Key size={15} />,
      content: <ApiKeysTab client={client} />,
    },
    {
      value: 'organizations',
      label: 'Organizations',
      icon: <Users size={15} />,
      content: <OrganizationsTab client={client} />,
    },
    {
      value: 'notifications',
      label: 'Notifications',
      icon: <Bell size={15} />,
      content: <NotificationsTab client={notificationsClient} />,
    },
  ];

  return (
    <Stack gap={8}>
      <PageHeader
        title="Account"
        subtitle="Manage your profile, security, API keys, and organizations."
      />
      <Tabs items={items} />
    </Stack>
  );
}
