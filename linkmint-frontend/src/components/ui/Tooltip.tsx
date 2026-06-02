'use client';

/**
 * Tooltip — a supplementary hint wrapping Chakra v3 `Tooltip.*` (frontendfeature.md §2.5),
 * encapsulating the Trigger → Portal → Positioner → Content → Arrow chain so callers pass just
 * `content` and a single focusable trigger child.
 *
 * a11y (F.6): opens on hover AND keyboard focus, closes on Esc/blur; content gets `role="tooltip"`
 * and is wired via `aria-describedby`. Use for non-essential hints only — never the sole carrier of
 * information. The trigger child must be a focusable element so keyboard users can reveal the hint.
 */

import type { ReactNode } from 'react';
import { Portal, Tooltip as TooltipNamespace } from '@chakra-ui/react';

export interface TooltipProps {
  /** Tooltip body. When absent, the children render with no tooltip (handy for conditional tips). */
  content?: ReactNode;
  /** The trigger element — must be focusable (a button/link). */
  children: ReactNode;
  /** @default 'top' */
  placement?: 'top' | 'bottom' | 'left' | 'right';
  /** Show the little arrow. @default true */
  showArrow?: boolean;
  /** Hover open delay (ms). @default 300 */
  openDelay?: number;
  /** Hover close delay (ms). @default 100 */
  closeDelay?: number;
  /** Disable the tooltip entirely. @default false */
  disabled?: boolean;
  /** Render into a portal to avoid clipping in overflow:hidden containers. @default true */
  portalled?: boolean;
}

export function Tooltip({
  content,
  children,
  placement = 'top',
  showArrow = true,
  openDelay = 300,
  closeDelay = 100,
  disabled = false,
  portalled = true,
}: TooltipProps) {
  if (content === undefined || content === null) {
    return <>{children}</>;
  }

  const positioner = (
    <TooltipNamespace.Positioner>
      <TooltipNamespace.Content
        bg="ink"
        color="surface"
        fontSize="xs"
        px={2.5}
        py={1.5}
        borderRadius="sm"
        boxShadow="md"
        maxW="240px"
      >
        {showArrow ? (
          <TooltipNamespace.Arrow>
            <TooltipNamespace.ArrowTip />
          </TooltipNamespace.Arrow>
        ) : null}
        {content}
      </TooltipNamespace.Content>
    </TooltipNamespace.Positioner>
  );

  return (
    <TooltipNamespace.Root
      openDelay={openDelay}
      closeDelay={closeDelay}
      disabled={disabled}
      positioning={{ placement }}
    >
      <TooltipNamespace.Trigger asChild>{children}</TooltipNamespace.Trigger>
      {portalled ? <Portal>{positioner}</Portal> : positioner}
    </TooltipNamespace.Root>
  );
}
