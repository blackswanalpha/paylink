'use client';

/** Wraps `paylinks.create` with loading/error state, a success toast, and wizard advancement. */

import { useCallback, useState } from 'react';
import { toast } from 'sonner';
import type { CreatePayLinkInput } from '@linkmint/sdk';
import { useAppStore } from '@/store/app';
import { toDisplayError, type DisplayError } from '@/lib/errors';
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
        toast.success('PayLink created', { description: result.pl_id });
        created(data);
      } catch (err) {
        const error = toDisplayError(err);
        setState({ status: 'error', error });
        toast.error(error.title, { description: error.message });
      }
    },
    [client, created],
  );

  return { state, create };
}
