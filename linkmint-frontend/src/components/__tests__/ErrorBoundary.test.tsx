/**
 * ErrorBoundary — proves the class boundary catches a child render crash, renders the branded
 * fallback with a copyable error id, and logs the crash through `reportError` exactly once.
 */

import { describe, it, expect, vi } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { reportError } from '@/lib/reportError';

vi.mock('@/lib/reportError', () => ({ reportError: vi.fn() }));

function Boom(): never {
  throw new Error('boom');
}

describe('ErrorBoundary', () => {
  it('renders the branded fallback and logs once when a child throws', () => {
    // React logs caught errors to console.error; silence it for this expected throw.
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    renderWithTheme(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copy error id' })).toBeInTheDocument();
    expect(reportError).toHaveBeenCalledTimes(1);

    consoleSpy.mockRestore();
  });
});
