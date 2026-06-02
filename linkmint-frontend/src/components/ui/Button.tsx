'use client';

/**
 * Button — the kit's single import surface for actions, re-exporting Chakra's themed `Button` /
 * `IconButton` (frontendfeature.md §2.5). The four required treatments map to Chakra props:
 *   - primary  → `<Button colorPalette="emerald">` (the app default; solid emerald)
 *   - outline  → `<Button variant="outline">`
 *   - ghost    → `<Button variant="ghost">`
 *   - gold     → `<GoldButton>` (celebratory — settlements/premium)
 *
 * Why GoldButton instead of `colorPalette="champagne"`: the Ivory theme's `champagne` ramp stops at
 * `600`, but Chakra's `colorPalette` resolves shades up to `950`, so a champagne palette renders
 * broken. GoldButton paints the celebratory look with the semantic `gold.*` tokens via style props
 * (dark ink text clears 4.5:1 on champagne.400). Extending the ramp to a full palette is a deliberate
 * out-of-scope follow-up (would also let `colorPalette="champagne"` work).
 *
 * a11y (F.6): native <button> + the global emerald focus ring (theme/system.ts); `loading` sets
 * aria-busy and disables; icon-only actions must use `IconButton` with an `aria-label`.
 */

import { Button, IconButton, type ButtonProps } from '@chakra-ui/react';

export { Button, IconButton };
export type { ButtonProps, IconButtonProps } from '@chakra-ui/react';

export type GoldButtonProps = ButtonProps;

/** Celebratory gold action (settlement success, premium). Uses `gold.*` tokens, not colorPalette. */
export function GoldButton(props: GoldButtonProps) {
  return (
    <Button
      variant="solid"
      bg="gold.solid"
      color="ink"
      _hover={{ bg: 'champagne.500' }}
      _active={{ bg: 'champagne.600' }}
      {...props}
    />
  );
}
