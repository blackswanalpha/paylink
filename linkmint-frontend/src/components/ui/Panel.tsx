'use client';

/** Panel — the base surface card: white, hairline border, soft shadow, large radius (§2.5). */

import { Box, type BoxProps } from '@chakra-ui/react';

export interface PanelProps extends BoxProps {
  /** Lift on hover (for clickable cards). */
  interactive?: boolean;
}

export function Panel({ interactive, children, ...rest }: PanelProps) {
  return (
    <Box
      bg="bg.panel"
      borderWidth="1px"
      borderColor="border"
      borderRadius="lg"
      boxShadow="sm"
      p={6}
      transitionProperty="transform, box-shadow"
      transitionDuration="lmBase"
      transitionTimingFunction="lmStandard"
      {...(interactive
        ? { cursor: 'pointer', _hover: { transform: 'translateY(-2px)', boxShadow: 'md' } }
        : {})}
      {...rest}
    >
      {children}
    </Box>
  );
}
