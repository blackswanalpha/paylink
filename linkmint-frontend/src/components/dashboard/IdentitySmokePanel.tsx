/**
 * IdentitySmokePanel — a read-only proof that the work08 SDK expansion reaches the backend through
 * the new gateway pass-through route. It runs server-side: `client.auth.login` → `client.users.me`
 * via the new SDK resources (see `@/lib/identity`), then renders the authenticated profile. If
 * identity-service is unreachable it renders a friendly message instead of crashing.
 *
 * This is intentionally minimal — the full auth & account experience is work09/work10. It previews
 * one read only to validate the SDK ↔ gateway ↔ identity-service path end-to-end.
 */

import { Badge, HStack, Stack, Text } from '@chakra-ui/react';
import { KeyValueRow, Panel } from '@/components/ui';
import { getDevIdentitySession } from '@/lib/identity';

export async function IdentitySmokePanel() {
  let content: React.ReactNode;
  try {
    const { profile } = await getDevIdentitySession();
    const roles = profile.user_roles.length > 0 ? profile.user_roles.join(', ') : '—';
    content = (
      <Stack gap={2}>
        <KeyValueRow label="Signed in as" value={profile.email ?? profile.user_id} />
        <KeyValueRow label="User ID" value={profile.user_id} mono />
        <KeyValueRow label="KYC tier" value={String(profile.kyc_tier)} />
        <KeyValueRow label="Status" value={profile.status} />
        <KeyValueRow label="Roles" value={roles} />
      </Stack>
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : 'identity-service is unreachable';
    content = (
      <Text fontSize="sm" color="fg.muted">
        Couldn’t reach identity-service through the gateway ({message}). Bring the stack up with
        “docker compose up -d” and reload.
      </Text>
    );
  }

  return (
    <Panel>
      <Stack gap={4}>
        <HStack justify="space-between" align="center">
          <Text fontFamily="heading" fontWeight="600" fontSize="lg">
            Identity
          </Text>
          <Badge colorPalette="emerald" variant="subtle">
            work08 · SDK smoke
          </Badge>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          Live auth.login → users.me through the new gateway pass-through route (RS256). The full
          auth &amp; account screens arrive with work09/10.
        </Text>
        {content}
      </Stack>
    </Panel>
  );
}
