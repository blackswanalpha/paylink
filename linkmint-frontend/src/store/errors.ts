/**
 * useErrorStore (Zustand) — app-wide overlay state for the two failures that must interrupt the whole
 * app rather than render in place: a 401 (session expired → re-auth) and a 402 `KYC_REQUIRED`
 * (verification required). `reportError` dispatches into this store via `getState()` (no hook needed),
 * and `<GlobalErrorOverlays/>` (mounted in Provider) subscribes and renders the modals.
 *
 * Mirrors the shape of `src/store/app.ts`: minimal state + plain action setters, no async.
 */

import { create } from 'zustand';
import type { DisplayError } from '@/lib/errors';

interface ErrorOverlayState {
  /** The 401 that triggered the re-auth prompt, or null when dismissed. */
  reauth: DisplayError | null;
  /** The 402 `KYC_REQUIRED` that triggered the KYC gate, or null when dismissed. */
  kyc: DisplayError | null;
  requireReauth: (error: DisplayError) => void;
  requireKyc: (error: DisplayError) => void;
  dismissReauth: () => void;
  dismissKyc: () => void;
}

export const useErrorStore = create<ErrorOverlayState>((set) => ({
  reauth: null,
  kyc: null,
  requireReauth: (error) => set({ reauth: error }),
  requireKyc: (error) => set({ kyc: error }),
  dismissReauth: () => set({ reauth: null }),
  dismissKyc: () => set({ kyc: null }),
}));
