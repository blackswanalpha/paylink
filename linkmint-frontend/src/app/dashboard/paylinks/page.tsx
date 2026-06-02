import { PayLinksManager } from '@/components/paylinks/PayLinksManager';
import { devCreatorAddr, mintDevJwt } from '@/lib/jwt';

// Mint a fresh dev JWT per request (server-side; the secret never reaches the browser) and pass the
// creator address so the page scopes to "your" PayLinks.
export const dynamic = 'force-dynamic';

export default function PayLinksPage() {
  const token = mintDevJwt();
  return <PayLinksManager initialToken={token} creatorAddr={devCreatorAddr()} />;
}
