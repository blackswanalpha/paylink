/**
 * Render helper that mounts components under the real Ivory Premium `system` (the same one the app
 * uses via Provider.tsx) — but with a lightweight `<ChakraProvider>` only, skipping the Emotion SSR
 * cache + service-worker cleanup in Provider.tsx, which need Next's router context and are flaky in
 * jsdom. Returns a `user` (user-event) alongside the testing-library result.
 */

import type { ReactElement, ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChakraProvider } from '@chakra-ui/react';
import { system } from '@/theme/system';

function ThemeWrapper({ children }: { children: ReactNode }) {
  return <ChakraProvider value={system}>{children}</ChakraProvider>;
}

export function renderWithTheme(ui: ReactElement, options?: Omit<RenderOptions, 'wrapper'>) {
  return {
    user: userEvent.setup(),
    ...render(ui, { wrapper: ThemeWrapper, ...options }),
  };
}

// Re-export the testing-library surface so tests import everything from one place.
export * from '@testing-library/react';
