'use client';

/**
 * Modal — an ergonomic, controlled wrapper over Chakra v3 `Dialog.*` (frontendfeature.md §2.5),
 * collapsing the Root/Backdrop/Positioner/Content/Header/Body/Footer/CloseTrigger compound into one
 * component. Used for create / confirm / detail surfaces (e.g. work11's create-PayLink modal).
 *
 * a11y (F.6): Chakra provides the focus trap, `role="dialog"` + `aria-modal`, Esc-to-close, scroll
 * lock, and labels the dialog from `Dialog.Title` — which we always render (visually hidden when no
 * `title`) so the dialog keeps an accessible name. The close button is labelled. The global emerald
 * focus ring (theme/system.ts) applies to every control inside.
 */

import type { ReactNode, RefObject } from 'react';
import { Dialog, IconButton, Portal } from '@chakra-ui/react';
import { X } from 'react-feather';

export type ModalSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'cover' | 'full';

export interface ModalProps {
  /** Controlled open state. */
  open: boolean;
  /** Called on any dismissal: backdrop click, Esc, or the close button. */
  onClose: () => void;
  /** Header title (Fraunces). When omitted, a hidden title keeps the dialog labelled. */
  title?: ReactNode;
  /** Optional sub-line under the title. */
  description?: ReactNode;
  children: ReactNode;
  /** Footer node — typically the action <Button>s; rendered right-aligned. */
  footer?: ReactNode;
  /** Dialog width. @default 'md' */
  size?: ModalSize;
  /** Hide the top-right close (X) button. @default false */
  hideCloseButton?: boolean;
  /** Block backdrop-click + Esc dismissal (e.g. a destructive action in flight). @default false */
  disableDismiss?: boolean;
  /** Element to focus when the dialog opens (else the first focusable). */
  initialFocusRef?: RefObject<HTMLElement | null>;
  /** Use 'alertdialog' for destructive confirmations. @default 'dialog' */
  role?: 'dialog' | 'alertdialog';
}

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
  hideCloseButton = false,
  disableDismiss = false,
  initialFocusRef,
  role = 'dialog',
}: ModalProps) {
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(e) => {
        if (!e.open) onClose();
      }}
      size={size}
      role={role}
      placement="center"
      scrollBehavior="inside"
      closeOnInteractOutside={!disableDismiss}
      closeOnEscape={!disableDismiss}
      initialFocusEl={initialFocusRef ? () => initialFocusRef.current : undefined}
    >
      <Portal>
        <Dialog.Backdrop bg="rgba(28, 26, 23, 0.45)" backdropFilter="blur(2px)" />
        <Dialog.Positioner>
          <Dialog.Content
            bg="bg.panel"
            color="fg"
            borderWidth="1px"
            borderColor="border"
            borderRadius="xl"
            boxShadow="lg"
          >
            <Dialog.Header pb={description ? 2 : 4}>
              <Dialog.Title
                fontFamily="heading"
                fontWeight="600"
                fontSize="lg"
                srOnly={title === undefined}
              >
                {title ?? 'Dialog'}
              </Dialog.Title>
              {description ? (
                <Dialog.Description color="fg.muted" fontSize="sm" mt={1}>
                  {description}
                </Dialog.Description>
              ) : null}
            </Dialog.Header>

            <Dialog.Body>{children}</Dialog.Body>

            {footer ? (
              <Dialog.Footer display="flex" justifyContent="flex-end" gap={3}>
                {footer}
              </Dialog.Footer>
            ) : null}

            {hideCloseButton ? null : (
              <Dialog.CloseTrigger asChild>
                <IconButton
                  aria-label="Close dialog"
                  variant="ghost"
                  size="sm"
                  position="absolute"
                  top="3"
                  insetEnd="3"
                >
                  <X size={18} />
                </IconButton>
              </Dialog.CloseTrigger>
            )}
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
