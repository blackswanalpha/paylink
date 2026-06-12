'use client';

/**
 * Button — the kit's single import surface for actions, re-exporting Chakra's themed `Button` /
 * `IconButton` (frontendfeature.md §2.5). The four required treatments map to Chakra props:
 *   - primary  → `<Button colorPalette="emerald">` (the app default; solid emerald)
 *   - outline  → `<Button variant="outline">`
 *   - ghost    → `<Button variant="ghost">`
 *   - gold     → `<GoldButton>` (celebratory — settlements/premium)
 *
 * GoldButton is `colorPalette="champagne"`: the theme ships the full champagne ramp plus its
 * palette-group semantics (`champagne.solid/contrast/…`, contrast = ink, which clears 4.5:1 on
 * champagne.400), so Chakra's solid recipe paints the celebratory look directly.
 *
 * a11y (F.6): native <button> + the global emerald focus ring (theme/system.ts); `loading` sets
 * aria-busy and disables; icon-only actions must use `IconButton` with an `aria-label`.
 */

import { Button, IconButton, type ButtonProps } from '@chakra-ui/react';

export { Button, IconButton };
export type { ButtonProps, IconButtonProps } from '@chakra-ui/react';

export type GoldButtonProps = ButtonProps;

/** Celebratory gold action (settlement success, premium). */
export function GoldButton(props: GoldButtonProps) {
  return <Button variant="solid" colorPalette="champagne" {...props} />;
}
