/**
 * /kitchen-sink — a dev-only gallery of the work03 component kit for visual + keyboard verification.
 * 404s in production (this is a developer tool, not a product surface). Mirrors the app's server-page
 * idiom (force-dynamic) and delegates rendering to a 'use client' gallery.
 */

import { notFound } from 'next/navigation';
import { KitchenSink } from '@/components/dev/KitchenSink';

export const dynamic = 'force-dynamic';

export default function KitchenSinkPage() {
  if (process.env.NODE_ENV === 'production') {
    notFound();
  }
  return <KitchenSink />;
}
