'use client';

/**
 * ErrorBoundary — a React class boundary (the only way to catch render/lifecycle exceptions) mounted
 * inside Provider so it catches client-tree render crashes while the Chakra theme is still alive,
 * rendering the same branded `ErrorFallback` as the route files. `try/catch` errors flow through
 * `reportError`; errors thrown *during render* flow through here. It also logs the crash through the
 * normal pipeline (normalized, silent — the fallback is already on screen).
 */

import { Component, type ErrorInfo, type ReactNode } from 'react';
import { newErrorId } from '@/lib/errors';
import { reportError } from '@/lib/reportError';
import { ErrorFallback } from '@/components/ErrorFallback';

interface ErrorBoundaryProps {
  children: ReactNode;
  /** Custom fallback renderer; receives the generated id + a reset fn. Defaults to ErrorFallback. */
  fallback?: (props: { id: string; reset: () => void }) => ReactNode;
}

interface ErrorBoundaryState {
  errorId: string | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { errorId: null };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { errorId: newErrorId() };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    // Normalize + log; silent so we don't also toast (the full-page fallback is already showing).
    reportError(error, { silent: true });
    if (process.env.NODE_ENV !== 'production') {
      console.error('ErrorBoundary caught:', error, info.componentStack);
    }
  }

  reset = (): void => {
    this.setState({ errorId: null });
  };

  override render(): ReactNode {
    const { errorId } = this.state;
    if (errorId !== null) {
      if (this.props.fallback) {
        return this.props.fallback({ id: errorId, reset: this.reset });
      }
      return <ErrorFallback id={errorId} onReset={this.reset} />;
    }
    return this.props.children;
  }
}
