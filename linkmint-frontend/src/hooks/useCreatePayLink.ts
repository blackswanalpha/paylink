'use client';

/** Wraps `paylinks.create` with a loading→success toast (work07), inline error, wizard advancement. */

import { useCallback, useState } from 'react';
import type { CreatePayLinkInput } from '@linkmint/sdk';
import { useAppStore } from '@/store/app';
import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import type { StepData } from '@/types/wizard';

export interface CreateValues {
  receiver: string;
  amount: number;
  currency: string;
  /** ISO 8601 expiry instant. */
  expiry: string;
  usage?: 'single' | 'multi';
}

type State = { status: 'idle' } | { status: 'loading' } | { status: 'error'; error: DisplayError };

export function useCreatePayLink() {
  const client = useAppStore((s) => s.client);
  const created = useAppStore((s) => s.created);
  const [state, setState] = useState<State>({ status: 'idle' });

  const create = useCallback(
    async (values: CreateValues): Promise<void> => {
      const c = client;
      if (!c) return;
      setState({ status: 'loading' });
      // Loading→success toast (work07). On failure we DISMISS this toast and let the error system
      // render the single surface inline (KycGate / ErrorBanner) — so there's no competing error
      // toast (F.5). The success transitions this same toast in place via its id.
      const toastId = notify.loading('Creating PayLink…');
      try {
        const input: CreatePayLinkInput = {
          receiver: values.receiver,
          amount: values.amount,
          expiry: values.expiry,
          currency: values.currency,
          ...(values.usage ? { usage: values.usage } : {}),
        };
        const result = await c.paylinks.create(input);
        const data: StepData = {
          plId: result.pl_id,
          amount: values.amount,
          currency: values.currency,
          receiver: values.receiver,
          initialStatus: result.status,
        };
        setState({ status: 'idle' });
        notify.success('PayLink created', { id: toastId, description: result.pl_id });
        created(data);
      } catch (err) {
        // Route through the error system (work04): surface inline on the form, so a 402 KYC_REQUIRED
        // renders the contextual KycGate while a 401 still escalates to the global re-auth modal.
        notify.dismiss(toastId);
        const { error } = reportError(err, {
          surface: 'inline',
          context: 'while creating a PayLink',
        });
        setState({ status: 'error', error });
      }
    },
    [client, created],
  );

  return { state, create };
}
