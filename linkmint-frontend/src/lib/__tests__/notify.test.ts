/**
 * notify — the governed toast wrapper (work07). Asserts each kind maps to the right Sonner call with
 * curated options, that `loading` returns the toast id (for in-place transition), and that `promise`
 * wires loading/success/error and returns the ORIGINAL promise so callers can still await it.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';

const toastMock = {
  success: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
  error: vi.fn(),
  loading: vi.fn(() => 'tid'),
  promise: vi.fn(),
  dismiss: vi.fn(),
};
vi.mock('sonner', () => ({ toast: toastMock }));

// Imported after the mock so it binds to the mocked `toast`.
const { notify } = await import('@/lib/notify');

beforeEach(() => {
  vi.clearAllMocks();
});

describe('notify', () => {
  it('maps each kind to the matching Sonner method with curated options', () => {
    notify.success('Saved', { description: 'ok', duration: 1000 });
    expect(toastMock.success).toHaveBeenCalledWith('Saved', { description: 'ok', duration: 1000 });

    notify.info('i');
    expect(toastMock.info).toHaveBeenCalledWith('i', {});

    notify.warning('w');
    expect(toastMock.warning).toHaveBeenCalledWith('w', {});

    notify.error('e', { description: 'bad' });
    expect(toastMock.error).toHaveBeenCalledWith('e', { description: 'bad' });
  });

  it('passes a single action through', () => {
    const onClick = vi.fn();
    notify.success('A', { action: { label: 'Undo', onClick } });
    expect(toastMock.success).toHaveBeenCalledWith('A', { action: { label: 'Undo', onClick } });
  });

  it('loading returns the toast id (for an in-place transition to success)', () => {
    expect(notify.loading('Creating…')).toBe('tid');
    expect(toastMock.loading).toHaveBeenCalledWith('Creating…', {});
  });

  it('success can target an existing toast id', () => {
    notify.success('Done', { id: 'tid', description: '0xpl' });
    expect(toastMock.success).toHaveBeenCalledWith('Done', { id: 'tid', description: '0xpl' });
  });

  it('promise wires loading/success/error and returns the original promise', async () => {
    const p = Promise.resolve({ pl_id: 'x' });
    const ret = notify.promise(p, {
      loading: 'L',
      success: (d) => `ok ${d.pl_id}`,
      error: 'E',
    });
    expect(ret).toBe(p);
    expect(toastMock.promise).toHaveBeenCalledTimes(1);
    const opts = toastMock.promise.mock.calls[0]?.[1] as {
      loading: string;
      success: (d: { pl_id: string }) => string;
      error: (e: unknown) => string;
    };
    expect(opts.loading).toBe('L');
    expect(opts.success({ pl_id: 'x' })).toBe('ok x');
    expect(opts.error(new Error('z'))).toBe('E');
    await ret;
  });

  it('dismiss delegates to Sonner', () => {
    notify.dismiss('id');
    expect(toastMock.dismiss).toHaveBeenCalledWith('id');
  });
});
