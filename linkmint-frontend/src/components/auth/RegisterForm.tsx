'use client';

/**
 * RegisterForm — create an account with email + password (+ confirm). Decoupled from login: on success
 * it routes to `/login` with a toast rather than auto-authenticating. Envelope errors are classified
 * silently and shown inline (`EMAIL_TAKEN` → a friendly message; validation → a banner).
 */

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Button, Input, Stack } from '@chakra-ui/react';
import { UserPlus } from 'react-feather';

import { register } from '@/lib/authClient';
import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { ErrorBanner, FormField } from '@/components/ui';

const EMAIL_RE = /^\S+@\S+\.\S+$/;

export function RegisterForm() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [touched, setTouched] = useState(false);
  const [banner, setBanner] = useState<DisplayError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const errors = {
    email: EMAIL_RE.test(email) ? '' : 'Enter a valid email address.',
    password: password.length >= 8 ? '' : 'Use at least 8 characters.',
    confirm: confirm === password ? '' : 'Passwords do not match.',
  };
  const valid = !errors.email && !errors.password && !errors.confirm;
  const show = (msg: string): string | undefined => (touched && msg ? msg : undefined);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setTouched(true);
    setBanner(null);
    if (!valid) {
      return;
    }
    setSubmitting(true);
    try {
      await register({ email: email.trim(), password });
      notify.success('Account created', { description: 'Sign in to continue.' });
      router.push('/login');
    } catch (err) {
      const { error } = reportError(err, { silent: true });
      if (error.code === 'EMAIL_TAKEN') {
        setBanner({
          ...error,
          title: 'Email in use',
          message: 'That email is already registered.',
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

        <FormField label="Email" required error={show(errors.email)}>
          <Input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
          />
        </FormField>

        <FormField
          label="Password"
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
          loadingText="Creating account…"
          gap={2}
          width="full"
        >
          <UserPlus size={18} /> Create account
        </Button>
      </Stack>
    </form>
  );
}
