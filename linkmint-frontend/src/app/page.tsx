import { PayLinkDemo } from '@/components/PayLinkDemo';
import { mintDevJwt } from '@/lib/jwt';

// Mint a fresh dev JWT per request (server-side; the secret never reaches the browser) rather than
// baking one in at build time.
export const dynamic = 'force-dynamic';

export default function Page() {
  const token = mintDevJwt();
  return <PayLinkDemo initialToken={token} />;
}
