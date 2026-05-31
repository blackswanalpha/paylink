/** Shared types for the create → instructions → status demo flow. */

import type { PayLinkStatus } from '@linkmint/sdk';

export type Step = 'create' | 'instructions' | 'status';

/** The minimal data threaded across steps after a PayLink is created. */
export interface StepData {
  /** 0x-prefixed PayLink id. */
  plId: string;
  /** Amount in integer minor units (echoed for the instructions/status display). */
  amount: number;
  currency: string;
  receiver: string;
  /** Status returned by create (typically CREATED, or PENDING once submitted on-chain). */
  initialStatus: PayLinkStatus;
}
