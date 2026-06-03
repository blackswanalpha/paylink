'use client';

/**
 * LoginForm — email + password sign-in with a TOTP MFA challenge step.
 *
 * Correctness constraint (F.5): identity-service signals MFA as a 401 with `code: "MFA_REQUIRED"` (and
 * `MFA_INVALID`). `classifyError` would force EVERY 401 to the global "Session expired" overlay — wrong
 * here — so this form reports with `{ silent: true }` and branches on `error.code`, surfacing the MFA
 * field / inline messages instead of the overlay. Only an unexpected error falls through to a banner.
 */

import { useState, type FormEvent } from 'react';
import NextLink from 'next/link';
import { useRouter } from 'next/navigation';
import { Button, Input, Link, Stack } from '@chakra-ui/react';
import { LogIn } from 'react-feather';

import { login } from '@/lib/authClient';
import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { ErrorBanner, FormField } from '@/components/ui';
import { MfaChallengeField } from './MfaChallengeField';

/** Resolve a safe post-login destination (same-origin path only — no open redirects). */
function safeNext(next: string | undefined): string {
  if (next && next.startsWith('/') && !next.startsWith('//')) {
    return next;
  }
  return '/account';
}

export function LoginForm({ next }: { next?: string }) {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [mfaRequired, setMfaRequired] = useState(false);
  const [mfaCode, setMfaCode] = useState('');
  const [mfaError, setMfaError] = useState<string | undefined>(undefined);
  const [banner, setBanner] = useState<DisplayError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitting(true);
    setBanner(null);
    setMfaError(undefined);
    try {
      await login({
        email: email.trim(),
        password,
        mfa_code: mfaRequired ? mfaCode.trim() : undefined,
      });
      notify.success('Signed in');
      router.replace(safeNext(next));
    } catch (err) {
      // silent: classify without dispatching, so a 401 MFA challenge never opens the reauth overlay.
      const { error } = reportError(err, { silent: true });
      switch (error.code) {
        case 'MFA_REQUIRED':
          setMfaRequired(true);
          if (mfaRequired) {
            setMfaError('Enter the 6-digit code from your authenticator.');
          }
          break;
        case 'MFA_INVALID':
          setMfaRequired(true);
          setMfaError('That code is invalid or expired. Try again.');
          break;
        case 'INVALID_CREDENTIALS':
        case 'USER_NOT_FOUND':
          setBanner({ ...error, title: 'Sign-in failed', message: 'Incorrect email or password.' });
          break;
        case 'USER_SUSPENDED':
          setBanner({ ...error, title: 'Account suspended', message: error.message });
          break;
        default:
          setBanner(error);
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate>
      <Stack gap={4}>
        {banner ? <ErrorBanner error={banner} /> : null}

        <FormField label="Email" required>
          <Input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            disabled={mfaRequired}
          />
        </FormField>

        <FormField label="Password" required>
          <Input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            autoComplete="current-password"
            disabled={mfaRequired}
          />
        </FormField>

        {!mfaRequired ? (
          <Link asChild color="accent.fg" fontSize="sm" alignSelf="flex-end" mt={-2}>
            <NextLink href="/forgot-password">Forgot password?</NextLink>
          </Link>
        ) : null}

        {mfaRequired ? (
          <MfaChallengeField value={mfaCode} onChange={setMfaCode} error={mfaError} autoFocus />
        ) : null}

        <Button
          type="submit"
          colorPalette="emerald"
          loading={submitting}
          loadingText={mfaRequired ? 'Verifying…' : 'Signing in…'}
          gap={2}
          width="full"
        >
          <LogIn size={18} /> {mfaRequired ? 'Verify & sign in' : 'Sign in'}
        </Button>
      </Stack>
    </form>
  );
}
