import { AccountScreen } from '@/components/account/AccountScreen';
import { ProtectedRoute } from '@/components/auth/ProtectedRoute';
import { AppShell } from '@/components/shell/AppShell';
import { mintDevJwt } from '@/lib/jwt';

export const dynamic = 'force-dynamic';

export default function AccountPage() {
  // The Notifications tab (and the Topbar bell) talk to notification-service, which is scoped by the
  // creator address the gateway injects from this HS256 dev token — the dashboard's token context,
  // distinct from the RS256 identity session the other tabs use. Minted server-side, per request.
  const notificationsToken = mintDevJwt();
  return (
    <ProtectedRoute>
      <AppShell>
        <AccountScreen notificationsToken={notificationsToken} />
      </AppShell>
    </ProtectedRoute>
  );
}
