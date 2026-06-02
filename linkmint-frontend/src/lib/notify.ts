/**
 * notify — the governed toast layer (FE work07).
 *
 * The ONE module (besides the `<Toaster/>` mount in Provider.tsx) that imports Sonner. Everything
 * else — including the error system's `reportError` toast surface (work04) — routes through here, so
 * toast styling/behaviour is unified and there's a single seam to evolve. Toasts are TRANSIENT
 * feedback only; persistent alerts live in the notification center (server-backed inbox).
 *
 * Accessibility (F.6): Sonner's region is an `aria-live` (polite) container and every toast carries
 * a close button (configured on the Toaster), and toasts are never the only signal (a copy success
 * also flips the inline check; an error toast is also recoverable from the inbox / inline banner).
 * Reduced motion is handled at the Toaster/CSS layer (Provider.tsx + globals.css), not per call.
 */

import { toast, type ExternalToast } from 'sonner';

export type NotifyKind = 'success' | 'info' | 'warning' | 'error';

/** A curated subset of Sonner's per-toast options. */
export interface NotifyOptions {
  /** Secondary line under the title. */
  description?: string;
  /** Auto-dismiss in ms; omit to use the Toaster default. `Infinity` requires manual dismiss. */
  duration?: number;
  /** A single inline action button. */
  action?: { label: string; onClick: () => void };
  /** Stable id to update/de-dupe an existing toast. */
  id?: string | number;
}

/** Loading/success/error copy for a promise toast. The functions receive the resolved/rejected value. */
export interface NotifyPromiseMessages<T> {
  loading: string;
  success: string | ((data: T) => string);
  error: string | ((err: unknown) => string);
  /** Optional shared description shown on every state. */
  description?: string;
}

function toExternal(opts?: NotifyOptions): ExternalToast {
  const ext: ExternalToast = {};
  if (!opts) return ext;
  if (opts.description !== undefined) ext.description = opts.description;
  if (opts.duration !== undefined) ext.duration = opts.duration;
  if (opts.id !== undefined) ext.id = opts.id;
  if (opts.action) ext.action = { label: opts.action.label, onClick: opts.action.onClick };
  return ext;
}

export const notify = {
  success(title: string, opts?: NotifyOptions): string | number {
    return toast.success(title, toExternal(opts));
  },
  info(title: string, opts?: NotifyOptions): string | number {
    return toast.info(title, toExternal(opts));
  },
  warning(title: string, opts?: NotifyOptions): string | number {
    return toast.warning(title, toExternal(opts));
  },
  error(title: string, opts?: NotifyOptions): string | number {
    return toast.error(title, toExternal(opts));
  },
  /**
   * Show a loading toast and return its id. Pass that id to a later `notify.success`/`notify.error`
   * (via `opts.id`) to transition the SAME toast in place, or to `notify.dismiss` to remove it (e.g.
   * when an error is surfaced elsewhere — inline — so the toast isn't a competing error surface).
   */
  loading(title: string, opts?: NotifyOptions): string | number {
    return toast.loading(title, toExternal(opts));
  },
  /**
   * Wrap an async action in a loading→success/error toast. Returns the ORIGINAL promise so callers
   * can still `await`/`catch` it (e.g. to also surface an inline error via the error system).
   */
  promise<T>(promise: Promise<T>, messages: NotifyPromiseMessages<T>): Promise<T> {
    toast.promise(promise, {
      loading: messages.loading,
      success: (data: T) =>
        typeof messages.success === 'function' ? messages.success(data) : messages.success,
      error: (err: unknown) =>
        typeof messages.error === 'function' ? messages.error(err) : messages.error,
      description: messages.description,
    });
    return promise;
  },
  dismiss(id?: string | number): void {
    toast.dismiss(id);
  },
};
