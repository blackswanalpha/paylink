/**
 * KycGate — the inline (contextual) 402 gate. Asserts it renders the verification CTA + a copyable
 * trace id and invokes the (seam) onVerify; it does NOT fake verification.
 */

import { describe, it, expect, vi } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { KycGate } from '@/components/KycGate';
import type { DisplayError } from '@/lib/errors';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const kycError: DisplayError = {
  kind: 'api',
  title: 'Verification required',
  message: 'Verify your identity to continue.',
  status: 402,
  code: 'KYC_REQUIRED',
  traceId: 'trace-402',
};

describe('KycGate', () => {
  it('renders the gate with a verification CTA and copyable trace', async () => {
    const onVerify = vi.fn();
    const { user } = renderWithTheme(<KycGate error={kycError} onVerify={onVerify} />);

    expect(screen.getByText('Verification required')).toBeInTheDocument();
    expect(screen.getByText('Verify your identity to continue.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copy trace id' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Verify identity' }));
    expect(onVerify).toHaveBeenCalledTimes(1);
  });
});
