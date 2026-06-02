'use client';

/**
 * GlobalErrorOverlays — the two app-wide error surfaces that must interrupt the whole app: a 401
 * session-expiry re-auth prompt and the default 402 `KYC_REQUIRED` gate. Both are driven by
 * `useErrorStore` (which `reportError` dispatches into) and mounted once in Provider.
 *
 * Phase-honesty (F.7): the CTAs are real, reachable, and announce the failure with a copyable
 * `trace_id`, but the destinations are SEAMS — the login screen is work09 and the KYC flow is work15,
 * so the buttons don't fake auth/verification; they currently just dismiss.
 */

import { Button, Stack, Text } from '@chakra-ui/react';
import { useErrorStore } from '@/store/errors';
import { Modal } from '@/components/ui/Modal';
import { CopyField } from '@/components/ui/CopyField';

export function GlobalErrorOverlays() {
  const reauth = useErrorStore((s) => s.reauth);
  const kyc = useErrorStore((s) => s.kyc);
  const dismissReauth = useErrorStore((s) => s.dismissReauth);
  const dismissKyc = useErrorStore((s) => s.dismissKyc);

  return (
    <>
      <Modal
        open={reauth !== null}
        onClose={dismissReauth}
        role="alertdialog"
        title="Session expired"
        description="Your session is no longer valid. Sign in again to continue."
        footer={
          <Button
            colorPalette="emerald"
            onClick={() => {
              // SEAM(work09): navigate to the login screen once auth exists. For now, dismiss.
              dismissReauth();
            }}
          >
            Sign in again
          </Button>
        }
      >
        <Stack gap={3}>
          <Text color="fg.muted" fontSize="sm">
            {reauth?.message ?? 'Please re-authenticate to continue.'}
          </Text>
          {reauth?.traceId ? (
            <CopyField value={reauth.traceId} label="trace id" mono variant="inline" size="sm" />
          ) : null}
        </Stack>
      </Modal>

      <Modal
        open={kyc !== null}
        onClose={dismissKyc}
        role="alertdialog"
        title="Verification required"
        description="This action needs identity verification before it can continue."
        footer={
          <>
            <Button variant="outline" onClick={dismissKyc}>
              Not now
            </Button>
            <Button
              colorPalette="emerald"
              onClick={() => {
                // SEAM(work15): open the KYC flow once it exists. For now, dismiss.
                dismissKyc();
              }}
            >
              Verify identity
            </Button>
          </>
        }
      >
        <Stack gap={3}>
          <Text color="fg.muted" fontSize="sm">
            {kyc?.message ?? 'Identity verification is required to proceed.'}
          </Text>
          {kyc?.traceId ? (
            <CopyField value={kyc.traceId} label="trace id" mono variant="inline" size="sm" />
          ) : null}
        </Stack>
      </Modal>
    </>
  );
}
