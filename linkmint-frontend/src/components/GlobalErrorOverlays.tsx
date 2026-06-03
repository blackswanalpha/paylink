'use client';

/**
 * GlobalErrorOverlays — the two app-wide error surfaces that must interrupt the whole app: a 401
 * session-expiry re-auth prompt and the default 402 `KYC_REQUIRED` gate. Both are driven by
 * `useErrorStore` (which `reportError` dispatches into) and mounted once in Provider.
 *
 * Phase-honesty (F.7): the CTAs are real and reachable. The 401 re-auth CTA clears the identity
 * session and routes to `/login?next=…` (work09). The 402 KYC CTA is still a SEAM — the KYC flow is
 * work15 — so it just dismisses for now without faking verification.
 */

import { usePathname, useRouter } from 'next/navigation';
import { Button, Stack, Text } from '@chakra-ui/react';
import { useErrorStore } from '@/store/errors';
import { useAuthStore } from '@/store/auth';
import { Modal } from '@/components/ui/Modal';
import { CopyField } from '@/components/ui/CopyField';

export function GlobalErrorOverlays() {
  const reauth = useErrorStore((s) => s.reauth);
  const kyc = useErrorStore((s) => s.kyc);
  const dismissReauth = useErrorStore((s) => s.dismissReauth);
  const dismissKyc = useErrorStore((s) => s.dismissKyc);
  const router = useRouter();
  const pathname = usePathname();

  function handleReauth() {
    // The session is dead — drop it and send the user to log in, returning them here afterwards.
    useAuthStore.getState().clearSession();
    dismissReauth();
    router.push(`/login?next=${encodeURIComponent(pathname ?? '/')}`);
  }

  return (
    <>
      <Modal
        open={reauth !== null}
        onClose={dismissReauth}
        role="alertdialog"
        title="Session expired"
        description="Your session is no longer valid. Sign in again to continue."
        footer={
          <Button colorPalette="emerald" onClick={handleReauth}>
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
