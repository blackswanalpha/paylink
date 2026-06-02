'use client';

/**
 * AmountDisplay — a monetary figure in the Ivory Premium style: currency in muted small-caps, the
 * number in the Fraunces display serif. Logic stays in integer minor units; `formatMinorUnits`
 * (lib/money) provides the accessible label.
 */

import { Text } from '@chakra-ui/react';
import { formatMinorUnits } from '@/lib/money';

export interface AmountDisplayProps {
  /** Integer minor units (e.g. cents). */
  amountMinor: number;
  currency: string;
  /** Visual size of the number. */
  size?: 'sm' | 'md' | 'lg' | 'xl';
}

const NUMBER_SIZE: Record<NonNullable<AmountDisplayProps['size']>, string> = {
  sm: 'md',
  md: 'lg',
  lg: '2xl',
  xl: '4xl',
};

export function AmountDisplay({ amountMinor, currency, size = 'md' }: AmountDisplayProps) {
  const major = (amountMinor / 100).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return (
    <Text as="span" whiteSpace="nowrap" aria-label={formatMinorUnits(amountMinor, currency)}>
      <Text
        as="span"
        fontSize="xs"
        fontWeight="600"
        letterSpacing="0.06em"
        color="fg.muted"
        textTransform="uppercase"
        mr={1.5}
      >
        {currency}
      </Text>
      <Text as="span" fontFamily="heading" fontWeight="600" fontSize={NUMBER_SIZE[size]}>
        {major}
      </Text>
    </Text>
  );
}
