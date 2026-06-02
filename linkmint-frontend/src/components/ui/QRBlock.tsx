'use client';

/**
 * QRBlock — a scannable QR for a PayLink URL/id, wrapping Chakra v3 `QrCode.*` (frontendfeature.md
 * §2.5). Renders black-on-white modules (never tinted — scannability) centered on a white tile, with
 * an optional PNG download and a `caption` slot designed to hold a <CopyField> so the payload is never
 * available only as a QR.
 *
 * a11y (F.6): the QR <svg> is exposed as `role="img"` with an `aria-label`; pair it with a CopyField
 * caption so the same value is reachable as copyable text.
 */

import type { ReactNode } from 'react';
import { Box, QrCode, Stack } from '@chakra-ui/react';
import { Download } from 'react-feather';
import { Button } from './Button';

export interface QRBlockProps {
  /** The encoded payload (e.g. the PayLink share URL). */
  value: string;
  /** Pixel size of the QR. @default 200 */
  size?: number;
  /** Accessible description of what the code encodes. */
  label: string;
  /** Optional content under the code — typically a <CopyField> carrying the same value. */
  caption?: ReactNode;
  /** Show a "Download PNG" trigger. @default false */
  downloadable?: boolean;
  /** Download filename (without extension). @default 'paylink-qr' */
  downloadFileName?: string;
  /** Error-correction level (higher tolerates more damage but is denser). @default 'M' */
  level?: 'L' | 'M' | 'Q' | 'H';
}

export function QRBlock({
  value,
  size = 200,
  label,
  caption,
  downloadable = false,
  downloadFileName = 'paylink-qr',
  level = 'M',
}: QRBlockProps) {
  return (
    <QrCode.Root value={value} encoding={{ ecc: level }}>
      <Stack gap={3} align="center">
        <Box
          bg="surface"
          p={4}
          borderWidth="1px"
          borderColor="border"
          borderRadius="lg"
          boxShadow="xs"
        >
          <QrCode.Frame role="img" aria-label={label} boxSize={`${size}px`}>
            <QrCode.Pattern />
          </QrCode.Frame>
        </Box>
        {caption ? <Box w="100%">{caption}</Box> : null}
        {downloadable ? (
          <QrCode.DownloadTrigger asChild fileName={`${downloadFileName}.png`} mimeType="image/png">
            <Button variant="outline" size="sm" gap={2}>
              <Download size={16} /> Download PNG
            </Button>
          </QrCode.DownloadTrigger>
        ) : null}
      </Stack>
    </QrCode.Root>
  );
}
