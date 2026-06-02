'use client';

/**
 * Avatar — a user/merchant avatar wrapping Chakra v3 `Avatar.*` (frontendfeature.md §2.5). Shows the
 * image when `src` loads, else initials derived from `name`, else a fallback `icon`.
 *
 * a11y (F.6): the image carries `alt={name}`; the fallback icon is decorative. Tinted with the
 * emerald accent so initials read clearly on the ivory canvas.
 */

import type { ReactNode } from 'react';
import { Avatar as AvatarNamespace } from '@chakra-ui/react';

export interface AvatarProps {
  /** Image URL; falls back to initials/icon when absent or it fails to load. */
  src?: string;
  /** Full name — used for the initials and the image alt text. */
  name?: string;
  /** Fallback icon when there's no name (react-feather, e.g. <User/>). */
  icon?: ReactNode;
  /** @default 'md' */
  size?: '2xs' | 'xs' | 'sm' | 'md' | 'lg' | 'xl' | '2xl';
  /** @default 'full' (circle) */
  shape?: 'full' | 'rounded';
}

export function Avatar({ src, name, icon, size = 'md', shape = 'full' }: AvatarProps) {
  return (
    <AvatarNamespace.Root
      size={size}
      shape={shape}
      bg="accent.subtle"
      color="accent.fg"
      fontFamily="heading"
      fontWeight="600"
    >
      <AvatarNamespace.Fallback name={name}>{name ? undefined : icon}</AvatarNamespace.Fallback>
      {src ? <AvatarNamespace.Image src={src} alt={name ?? ''} /> : null}
    </AvatarNamespace.Root>
  );
}
