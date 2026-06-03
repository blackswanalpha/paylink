'use client';

/**
 * MfaEnrollModal — TOTP enrollment in two steps inside one dialog: on open it calls `auth.mfaEnroll`
 * and renders the `otpauth` QR (plus the raw secret as a CopyField backstop, F.6), then the user
 * enters a code to `auth.mfaVerify`. Reuses the authed identity client. Errors classify silently
 * (an invalid code is an inline field error, never the global overlay).
 */

import { useEffect, useState } from 'react';
import { Button, Stack } from '@chakra-ui/react';
import type { MfaEnrollResult } from '@linkmint/sdk';

import { createAuthedLinkMintClient } from '@/lib/linkmint';
import type { DisplayError } from '@/lib/errors';
import { notify } from '@/lib/notify';
import { reportError } from '@/lib/reportError';
import { CopyField, ErrorBanner, FormSkeleton, Modal, QRBlock } from '@/components/ui';
import { MfaChallengeField } from './MfaChallengeField';

export interface MfaEnrollModalProps {
  open: boolean;
  onClose: () => void;
  /** Called after a successful verify so the parent can refresh its MFA-enabled state. */
  onEnrolled: () => void;
}

export function MfaEnrollModal({ open, onClose, onEnrolled }: MfaEnrollModalProps) {
  const [enroll, setEnroll] = useState<MfaEnrollResult | null>(null);
  const [loadingEnroll, setLoadingEnroll] = useState(false);
  const [enrollError, setEnrollError] = useState<DisplayError | null>(null);
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState<string | undefined>(undefined);
  const [verifying, setVerifying] = useState(false);

  useEffect(() => {
    if (!open) {
      // Reset on close so a re-open starts a fresh enrollment.
      setEnroll(null);
      setEnrollError(null);
      setCode('');
      setCodeError(undefined);
      return;
    }
    let active = true;
    setLoadingEnroll(true);
    setEnrollError(null);
    createAuthedLinkMintClient()
      .auth.mfaEnroll()
      .then((result) => {
        if (active) {
          setEnroll(result);
        }
      })
      .catch((err) => {
        if (active) {
          const { error } = reportError(err, { silent: true });
          setEnrollError(error);
        }
      })
      .finally(() => {
        if (active) {
          setLoadingEnroll(false);
        }
      });
    return () => {
      active = false;
    };
  }, [open]);

  async function verify() {
    setVerifying(true);
    setCodeError(undefined);
    try {
      await createAuthedLinkMintClient().auth.mfaVerify({ code: code.trim() });
      notify.success('Two-factor authentication enabled');
      onEnrolled();
      onClose();
    } catch (err) {
      const { error } = reportError(err, { silent: true });
      setCodeError(
        error.code === 'MFA_INVALID'
          ? 'That code is invalid or expired. Try again.'
          : error.message,
      );
    } finally {
      setVerifying(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Enable two-factor authentication"
      description="Scan the QR with your authenticator app, then enter the 6-digit code to confirm."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            colorPalette="emerald"
            onClick={verify}
            loading={verifying}
            loadingText="Verifying…"
            disabled={!enroll || code.trim().length < 6}
          >
            Verify &amp; enable
          </Button>
        </>
      }
    >
      {loadingEnroll ? (
        <FormSkeleton />
      ) : enrollError ? (
        <ErrorBanner error={enrollError} />
      ) : enroll ? (
        <Stack gap={5}>
          <QRBlock
            value={enroll.otpauth_uri}
            label="Two-factor enrollment QR code"
            caption={
              <CopyField value={enroll.secret} label="setup key" mono variant="block" size="sm" />
            }
          />
          <MfaChallengeField
            value={code}
            onChange={setCode}
            error={codeError}
            label="Verification code"
          />
        </Stack>
      ) : null}
    </Modal>
  );
}
