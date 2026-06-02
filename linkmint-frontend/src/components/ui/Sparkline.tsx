'use client';

/**
 * Sparkline — a zero-dependency inline-SVG trend line with a soft area fill (§2.5). Emerald stroke
 * on an emerald-tinted gradient. Decorative, so it is aria-hidden; the figure it accompanies carries
 * the accessible value.
 */

import { useId } from 'react';
import { Box } from '@chakra-ui/react';

export interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  strokeColor?: string;
}

export function Sparkline({
  data,
  width = 160,
  height = 44,
  strokeColor = '#0F6E4E',
}: SparklineProps) {
  const gradientId = useId();
  const pad = 2;

  if (data.length === 0) {
    return <Box w={`${width}px`} h={`${height}px`} aria-hidden />;
  }

  const max = Math.max(...data);
  const min = Math.min(...data);
  const span = max - min || 1;
  const stepX = data.length > 1 ? (width - pad * 2) / (data.length - 1) : 0;

  const points = data.map((value, i) => {
    const x = pad + i * stepX;
    const y = pad + (height - pad * 2) * (1 - (value - min) / span);
    return [x, y] as const;
  });

  const line = points
    .map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)} ${y.toFixed(1)}`)
    .join(' ');
  const lastX = points[points.length - 1]?.[0] ?? pad;
  const firstX = points[0]?.[0] ?? pad;
  const area = `${line} L${lastX.toFixed(1)} ${height - pad} L${firstX.toFixed(1)} ${height - pad} Z`;

  return (
    <Box as="span" display="inline-block" aria-hidden>
      <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="presentation">
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={strokeColor} stopOpacity="0.18" />
            <stop offset="100%" stopColor={strokeColor} stopOpacity="0" />
          </linearGradient>
        </defs>
        <path d={area} fill={`url(#${gradientId})`} stroke="none" />
        <path
          d={line}
          fill="none"
          stroke={strokeColor}
          strokeWidth="1.75"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </Box>
  );
}
