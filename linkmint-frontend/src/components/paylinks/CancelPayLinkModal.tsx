'use client';

/**
 * CancelPayLinkModal — the confirm dialog for the destructive PayLink cancel (work06). Controlled by
 * `target` (null = closed). Uses `role="alertdialog"` for the destructive confirmation (F.6); the
 * parent owns the actual mutation (`usePayLinks().cancel`) so this stays presentational and shared by
 * the dashboard and the /dashboard/paylinks page.
 */

import { HStack, Text } from '@chakra-ui/react';
import type { PayLink } from '@linkmint/sdk';
import { AddressChip, Button, Modal } from '@/components/ui';

export interface CancelPayLinkModalProps {
  /** The PayLink to cancel, or null when the dialog is closed. */
  target: PayLink | null;
  onClose: () => void;
  onConfirm: (pl: PayLink) => void;
}

export function CancelPayLinkModal({ target, onClose, onConfirm }: CancelPayLinkModalProps) {
  return (
    <Modal
      open={target !== null}
      onClose={onClose}
      role="alertdialog"
      size="sm"
      title="Cancel this PayLink?"
      description="Cancelling is permanent — the PayLink can no longer be paid, and this can't be undone."
      footer={
        <>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Keep it
          </Button>
          <Button
            variant="solid"
            colorPalette="red"
            size="sm"
            onClick={() => {
              if (target) {
                onConfirm(target);
              }
            }}
          >
            Cancel PayLink
          </Button>
        </>
      }
    >
      {target ? (
        <HStack gap={2}>
          <Text fontSize="sm" color="fg.muted">
            PayLink
          </Text>
          <AddressChip value={target.pl_id} label="PayLink id" />
        </HStack>
      ) : null}
    </Modal>
  );
}
