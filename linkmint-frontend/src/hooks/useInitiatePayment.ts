'use client';

/**
 * Best-effort `payments.initiate` for the chosen rail (mpesa), fired once when the instructions
 * view mounts. The expected work35 outcome (`PAYLINK_NOT_PAYABLE` on a PENDING PayLink) is treated
 * as a non-fatal, clearly-labeled note — the PayLink poll remains the settlement source of truth.
 *
 * A stable idempotency key (per PayLink) makes the call safe under React Strict Mode's double-mount.
 */

import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { ConflictError } from '@linkmint/sdk';
import type { Payment } from '@linkmint/sdk';
import { useAppStore } from '@/store/app';
import { toDisplayError, type DisplayError } from '@/lib/errors';

export type InitiateState =
  | { status: 'idle' | 'loading' | 'not_payable' }
  | { status: 'recorded'; payment: Payment }
  | { status: 'error'; error: DisplayError };

export function useInitiatePayment(plId: string): InitiateState {
  const client = useAppStore((s) => s.client);
  const [state, setState] = useState<InitiateState>({ status: 'idle' });

  useEffect(() => {
    const c = client;
    if (!c) return;
    let cancelled = false;
    setState({ status: 'loading' });

    c.payments
      .initiate({ paylink_id: plId, rail: 'mpesa' }, { idempotencyKey: `web-initiate:${plId}` })
      .then((payment) => {
        if (cancelled) return;
        setState({ status: 'recorded', payment });
        toast.success('Payment intent recorded', { description: `rail: mpesa · ${payment.id}` });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ConflictError && err.code === 'PAYLINK_NOT_PAYABLE') {
          setState({ status: 'not_payable' });
          toast.info('Payment intent not recorded', {
            description:
              'Known work35 limitation — the PayLink is already PENDING on-chain. Settlement is tracked from the PayLink itself.',
          });
          return;
        }
        const error = toDisplayError(err);
        setState({ status: 'error', error });
        toast.warning(error.title, { description: error.message });
      });

    return () => {
      cancelled = true;
    };
  }, [client, plId]);

  return state;
}
