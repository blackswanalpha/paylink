'use client';

/**
 * Drawer — a controlled side/sheet panel wrapping Chakra v3 `Drawer.*` (frontendfeature.md §2.5).
 * Shares Modal's prop shape (so the two are interchangeable) and adds `placement`. Used for detail
 * panels and filters (e.g. a PayLink detail drawer in work11).
 *
 * a11y (F.6): same focus-trap / `role="dialog"` / Esc-to-close / labelled-by-title guarantees as
 * Modal — `Drawer.Title` is always rendered (hidden when no `title`). Close button is labelled.
 */

import type { ReactNode, RefObject } from 'react';
import { Drawer as DrawerNamespace, IconButton, Portal } from '@chakra-ui/react';
import { X } from 'react-feather';

export type DrawerPlacement = 'start' | 'end' | 'top' | 'bottom';
export type DrawerSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'full';

export interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  /** Edge the panel slides from. 'end' = right in LTR. @default 'end' */
  placement?: DrawerPlacement;
  /** Panel size. @default 'md' */
  size?: DrawerSize;
  hideCloseButton?: boolean;
  disableDismiss?: boolean;
  initialFocusRef?: RefObject<HTMLElement | null>;
}

export function Drawer({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  placement = 'end',
  size = 'md',
  hideCloseButton = false,
  disableDismiss = false,
  initialFocusRef,
}: DrawerProps) {
  return (
    <DrawerNamespace.Root
      open={open}
      onOpenChange={(e) => {
        if (!e.open) onClose();
      }}
      placement={placement}
      size={size}
      closeOnInteractOutside={!disableDismiss}
      closeOnEscape={!disableDismiss}
      initialFocusEl={initialFocusRef ? () => initialFocusRef.current : undefined}
    >
      <Portal>
        <DrawerNamespace.Backdrop bg="rgba(28, 26, 23, 0.45)" backdropFilter="blur(2px)" />
        <DrawerNamespace.Positioner>
          <DrawerNamespace.Content bg="bg.panel" color="fg" borderColor="border" boxShadow="lg">
            <DrawerNamespace.Header
              borderBottomWidth="1px"
              borderColor="border"
              pb={description ? 3 : 4}
            >
              <DrawerNamespace.Title
                fontFamily="heading"
                fontWeight="600"
                fontSize="lg"
                srOnly={title === undefined}
              >
                {title ?? 'Panel'}
              </DrawerNamespace.Title>
              {description ? (
                <DrawerNamespace.Description color="fg.muted" fontSize="sm" mt={1}>
                  {description}
                </DrawerNamespace.Description>
              ) : null}
            </DrawerNamespace.Header>

            <DrawerNamespace.Body>{children}</DrawerNamespace.Body>

            {footer ? (
              <DrawerNamespace.Footer
                borderTopWidth="1px"
                borderColor="border"
                display="flex"
                justifyContent="flex-end"
                gap={3}
              >
                {footer}
              </DrawerNamespace.Footer>
            ) : null}

            {hideCloseButton ? null : (
              <DrawerNamespace.CloseTrigger asChild>
                <IconButton
                  aria-label="Close panel"
                  variant="ghost"
                  size="sm"
                  position="absolute"
                  top="3"
                  insetEnd="3"
                >
                  <X size={18} />
                </IconButton>
              </DrawerNamespace.CloseTrigger>
            )}
          </DrawerNamespace.Content>
        </DrawerNamespace.Positioner>
      </Portal>
    </DrawerNamespace.Root>
  );
}
