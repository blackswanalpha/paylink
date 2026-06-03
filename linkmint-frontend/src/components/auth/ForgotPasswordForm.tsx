'use client';

/**
 * ForgotPasswordForm — request a password-reset link by email.
 *
 * Anti-enumeration: whatever the outcome (account exists, doesn't, or the service errors), the form
 * settles into the SAME confirmation — it never reveals whether an address is registered. In dev the
 * response may include a `reset_token`; when present we surface a clickable link to the reset screen
 * so the flow is testable without an email rail (this is null, and the link absent, in production).
 */

import { useState, type FormEvent } from 'react';
import NextLink from 'next/link';
import { Box, Button, Input, Link, Stack, Text } from '@chakra-ui/react';
import { Mail } from 'react-feather';

import { requestPasswordReset } from '@/lib/authClient';
import { reportError } from '@/lib/reportError';
import { FormField } from '@/components/ui';

const EMAIL_RE = /^\S+@\S+\.\S+$/;

export function ForgotPasswordForm() {
  const [email, setEmail] = useState('');
  const [touched, setTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);
  const [devToken, setDevToken] = useState<string | null>(null);

  const emailError = EMAIL_RE.test(email) ? '' : 'Enter a valid email address.';
  const show = (msg: string): string | undefined => (touched && msg ? msg : undefined);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setTouched(true);
    if (emailError) {
      return;
    }
    setSubmitting(true);
    try {
      const result = await requestPasswordReset({ email: email.trim() });
      setDevToken(result.reset_token ?? null);
    } catch (err) {
      // Don't surface anything that could reveal account existence — log silently and still confirm.
      reportError(err, { silent: true });
    } finally {
      setSubmitting(false);
      setDone(true);
    }
  }

  if (done) {
    return (
      <Stack gap={4}>
        <Text fontSize="sm" color="fg.muted">
          If an account exists for <strong>{email.trim()}</strong>, we&apos;ve sent a link to reset
          your password. Check your inbox and follow the instructions.
        </Text>
        {devToken ? (
          <Box borderWidth="1px" borderColor="border.muted" borderRadius="md" bg="bg.subtle" p={3}>
            <Text fontSize="xs" color="fg.muted" mb={1}>
              Dev only — no email is sent locally. Continue the reset:
            </Text>
            <Link asChild color="accent.fg" fontWeight="500" fontSize="sm">
              <NextLink href={`/reset-password?token=${encodeURIComponent(devToken)}`}>
                Open reset link
              </NextLink>
            </Link>
          </Box>
        ) : null}
      </Stack>
    );
  }

  return (
    <form onSubmit={onSubmit} noValidate>
      <Stack gap={4}>
        <Text fontSize="sm" color="fg.muted">
          Enter your account email and we&apos;ll send a link to reset your password.
        </Text>

        <FormField label="Email" required error={show(emailError)}>
          <Input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
          />
        </FormField>

        <Button
          type="submit"
          colorPalette="emerald"
          loading={submitting}
          loadingText="Sending link…"
          gap={2}
          width="full"
        >
          <Mail size={18} /> Send reset link
        </Button>
      </Stack>
    </form>
  );
}
