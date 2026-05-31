import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { Provider } from '@/components/ui/Provider';
import './globals.css';

export const metadata: Metadata = {
  title: 'LinkMint — PayLink demo',
  description: 'Create a PayLink, pay via M-PESA, and watch it settle on-chain.',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
