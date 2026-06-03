'use client';

/**
 * ProfileTab — read-only identity facts (user id, KYC tier, status, member-since) plus an editable
 * email/phone form (`users.updateMe`). On save the store profile is patched. Envelope errors
 * (`EMAIL_TAKEN`/`PHONE_TAKEN`) classify silently to a friendly inline banner.
 */

import { useState, type FormEvent } from 'react';
import { Box, Input, Stack, Text } from '@chakra-ui/react';
import type { LinkMintClient } from '@linkmint/sdk';

import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { useAuthStore } from '@/store/auth';
import { Button, ErrorBanner, FormField, FormSkeleton, KeyValueRow, Panel } from '@/components/ui';

function formatDate(iso: string): string {
  const t = Date.parse(iso);
  return Number.isNaN(t) ? '—' : new Date(t).toLocaleDateString();
}

export function ProfileTab({ client }: { client: LinkMintClient }) {
  const user = useAuthStore((s) => s.user);
  const patchUser = useAuthStore((s) => s.patchUser);
  const [email, setEmail] = useState(user?.email ?? '');
  const [phone, setPhone] = useState(user?.phone ?? '');
  const [saving, setSaving] = useState(false);
  const [banner, setBanner] = useState<DisplayError | null>(null);

  if (!user) {
    return <FormSkeleton />;
  }

  const dirty = email !== (user.email ?? '') || phone !== (user.phone ?? '');

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setBanner(null);
    try {
      const updated = await client.users.updateMe({
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
      });
      patchUser(updated);
      notify.success('Profile updated');
    } catch (err) {
      const { error } = reportError(err, { silent: true });
      if (error.code === 'EMAIL_TAKEN') {
        setBanner({ ...error, title: 'Email in use', message: 'That email is already in use.' });
      } else if (error.code === 'PHONE_TAKEN') {
        setBanner({ ...error, title: 'Phone in use', message: 'That phone is already in use.' });
      } else {
        setBanner(error);
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <Stack gap={6} pt={2}>
      <Panel>
        <Stack gap={3}>
          <Text fontFamily="heading" fontWeight="600">
            Account
          </Text>
          <KeyValueRow label="User ID" value={user.user_id} mono />
          <KeyValueRow label="KYC tier" value={`Tier ${user.kyc_tier}`} />
          <KeyValueRow label="Status" value={user.status} />
          <KeyValueRow label="Member since" value={formatDate(user.created_at)} />
        </Stack>
      </Panel>

      <Panel>
        <form onSubmit={onSubmit} noValidate>
          <Stack gap={4}>
            <Text fontFamily="heading" fontWeight="600">
              Profile
            </Text>
            {banner ? <ErrorBanner error={banner} /> : null}
            <FormField label="Email">
              <Input
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                type="email"
                autoComplete="email"
                placeholder="you@example.com"
              />
            </FormField>
            <FormField label="Phone" helperText="Optional.">
              <Input
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                type="tel"
                placeholder="+254700000000"
              />
            </FormField>
            <Box>
              <Button
                type="submit"
                colorPalette="emerald"
                loading={saving}
                loadingText="Saving…"
                disabled={!dirty}
              >
                Save changes
              </Button>
            </Box>
          </Stack>
        </form>
      </Panel>
    </Stack>
  );
}
