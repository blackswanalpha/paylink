import { MerchantDashboard } from '@/components/dashboard/MerchantDashboard';
import { IdentitySmokePanel } from '@/components/dashboard/IdentitySmokePanel';
import { devCreatorAddr, mintDevJwt } from '@/lib/jwt';

// Mint a fresh dev JWT per request (server-side; the secret never reaches the browser) and pass the
// creator address so the dashboard scopes to "your" PayLinks.
export const dynamic = 'force-dynamic';

export default function DashboardPage() {
  const token = mintDevJwt();
  // The identity card is a server component (work08 smoke): it logs in via the new SDK auth resource
  // and reads users.me through the gateway pass-through route. Passed as a slot so it renders inside
  // the dashboard shell.
  return (
    <MerchantDashboard
      initialToken={token}
      creatorAddr={devCreatorAddr()}
      slot={<IdentitySmokePanel />}
    />
  );
}
