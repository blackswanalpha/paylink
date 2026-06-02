import { MerchantDashboard } from '@/components/dashboard/MerchantDashboard';
import { devCreatorAddr, mintDevJwt } from '@/lib/jwt';

// Mint a fresh dev JWT per request (server-side; the secret never reaches the browser) and pass the
// creator address so the dashboard scopes to "your" PayLinks.
export const dynamic = 'force-dynamic';

export default function DashboardPage() {
  const token = mintDevJwt();
  return <MerchantDashboard initialToken={token} creatorAddr={devCreatorAddr()} />;
}
