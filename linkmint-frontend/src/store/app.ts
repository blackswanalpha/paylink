/**
 * Global app store (Zustand): the SDK client (built once from the server-minted token) and the
 * 3-step wizard navigation. Per-call async (create/initiate/poll) lives in the hooks, not here.
 */

import { create } from 'zustand';
import type { LinkMintClient } from '@linkmint/sdk';
import type { Step, StepData } from '@/types/wizard';

interface AppState {
  /** The single SDK client; null until initialized client-side from the dev token. */
  client: LinkMintClient | null;
  setClient: (client: LinkMintClient) => void;

  step: Step;
  /** Data for the active PayLink; null on the create step. */
  data: StepData | null;

  /** Move to the instructions step with the freshly created PayLink. */
  created: (data: StepData) => void;
  /** Advance from instructions to the live settlement view. */
  proceedToStatus: () => void;
  /** Start over with a new PayLink. */
  reset: () => void;
}

export const useAppStore = create<AppState>((set) => ({
  client: null,
  setClient: (client) => set({ client }),

  step: 'create',
  data: null,

  created: (data) => set({ step: 'instructions', data }),
  proceedToStatus: () =>
    set((state) => (state.step === 'instructions' ? { step: 'status' } : state)),
  reset: () => set({ step: 'create', data: null }),
}));
