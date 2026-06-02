import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Provider } from '@/components/ui/Provider';
import './globals.css';

export const metadata: Metadata = {
  title: 'LinkMint — Pay anyone, through any rail, with a link',
  description:
    'LinkMint turns payments into programmable, shareable links. Create a PayLink, pay via any rail, and watch it settle on-chain — non-custodial, instant, verifiable.',
};

// Fraunces (display) + Inter (UI/body) + JetBrains Mono (hashes), loaded via Google Fonts with
// display=swap. The theme (src/theme/system.ts) references these families with robust fallbacks, so
// a render without network still degrades gracefully.
const FONTS_HREF =
  'https://fonts.googleapis.com/css2?' +
  'family=Fraunces:opsz,wght@9..144,400;9..144,500;9..144,600;9..144,700&' +
  'family=Inter:wght@400;500;600;700&' +
  'family=JetBrains+Mono:wght@400;500&display=swap';

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link rel="stylesheet" href={FONTS_HREF} />
      </head>
      <body>
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
