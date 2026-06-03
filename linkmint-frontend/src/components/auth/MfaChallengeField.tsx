'use client';

/**
 * MfaChallengeField — the 6-digit TOTP code input, shared by the login MFA challenge and the enroll
 * modal's verify step. A plain controlled field wrapped in the kit's FormField (label + inline error).
 */

import { Input } from '@chakra-ui/react';

import { FormField } from '@/components/ui';

export interface MfaChallengeFieldProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
  /** @default 'Authentication code' */
  label?: string;
  autoFocus?: boolean;
}

export function MfaChallengeField({
  value,
  onChange,
  error,
  label = 'Authentication code',
  autoFocus = false,
}: MfaChallengeFieldProps) {
  return (
    <FormField
      label={label}
      required
      error={error}
      helperText="6-digit code from your authenticator app."
    >
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value.replace(/\s/g, ''))}
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        placeholder="123456"
        fontFamily="mono"
        autoFocus={autoFocus}
      />
    </FormField>
  );
}
