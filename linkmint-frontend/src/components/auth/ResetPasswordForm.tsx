'use client';

/**
 * ResetPasswordForm — set a new password using a reset token from the emailed link
 * (`/reset-password?token=…`). Mirrors RegisterForm's validation (≥8 chars + confirm match). On
 * success it routes to `/login` with a toast; the user then signs in with the new password (every
 * prior session was revoked server-side). An invalid/expired token surfaces inline with a path back
 * to request a fresh link.
 */

import { useState, type FormEvent } from 'react';
import NextLink from 'next/link';
import { useRouter } from 'next/navigation';
import { Button, Input, Link, Stack, Text } from '@chakra-ui/react';
import { Lock } from 'react-feather';

import { confirmPasswordReset } from '@/lib/authClient';
import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { ErrorBanner, FormField } from '@/components/ui';

export function ResetPasswordForm({ token }: { token?: string }) {
  const router = useRouter();
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [touched, setTouched] = useState(false);
  const [banner, setBanner] = useState<DisplayError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  if (!token) {
    return (
      <Stack gap={4}>
        <Text fontSize="sm" color="fg.muted">
          This reset link is missing its token. Request a new one to continue.
        </Text>
        <Link asChild color="accent.fg" fontWeight="500" fontSize="sm">
          <NextLink href="/forgot-password">Request a new link</NextLink>
        </Link>
      </Stack>
    );
  }

  const errors = {
    password: password.length >= 8 ? '' : 'Use at least 8 characters.',
    confirm: confirm === password ? '' : 'Passwords do not match.',
  };
  const valid = !errors.password && !errors.confirm;
  const show = (msg: string): string | undefined => (touched && msg ? msg : undefined);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setTouched(true);
    setBanner(null);
    const resetToken = token;
    if (!valid || !resetToken) {
      return;
    }
    setSubmitting(true);
    try {
      await confirmPasswordReset({ token: resetToken, new_password: password });
      notify.success('Password reset', { description: 'Sign in with your new password.' });
      router.push('/login');
    } catch (err) {
      const { error } = reportError(err, { silent: true });
      if (error.code === 'INVALID_TOKEN' || error.code === 'TOKEN_EXPIRED') {
        setBanner({
          ...error,
          title: 'Link expired',
          message: 'This reset link is invalid or has expired. Request a new one below.',
        });
      } else {
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

        <FormField
          label="New password"
          required
          helperText="At least 8 characters."
          error={show(errors.password)}
        >
          <Input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            autoComplete="new-password"
          />
        </FormField>

        <FormField label="Confirm password" required error={show(errors.confirm)}>
          <Input
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            type="password"
            autoComplete="new-password"
          />
        </FormField>

        <Button
          type="submit"
          colorPalette="emerald"
          loading={submitting}
          loadingText="Resetting…"
          gap={2}
          width="full"
        >
          <Lock size={18} /> Reset password
        </Button>

        <Link asChild color="fg.muted" fontSize="sm" alignSelf="center">
          <NextLink href="/forgot-password">Request a new link</NextLink>
        </Link>
      </Stack>
    </form>
  );
}
