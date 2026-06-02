'use client';

/**
 * Stepper — a presentational step indicator (numbered dots, connector line, complete/current/upcoming
 * states) over Chakra v3 `Steps.*` (frontendfeature.md §2.5). Driven entirely by `current`; it renders
 * no panel content and performs no navigation, so wizards (work07 PayLink flow, work14 onboarding) keep
 * owning their step state and just pass it here.
 *
 * a11y (F.6): the active step carries `aria-current="step"`, and every step has a visually-hidden status
 * ("completed" / "current" / "upcoming") so state is conveyed without relying on color. Completed steps
 * show a check glyph (a non-color cue) in place of the number.
 */

import type { ReactNode } from 'react';
import { Box, Steps } from '@chakra-ui/react';
import { Check } from 'react-feather';

export interface StepDescriptor {
  title: ReactNode;
  description?: ReactNode;
}

export interface StepperProps {
  steps: StepDescriptor[];
  /** Zero-based index of the current step. */
  current: number;
  /** @default 'horizontal' */
  orientation?: 'horizontal' | 'vertical';
  /** @default 'md' */
  size?: 'sm' | 'md' | 'lg';
}

function statusLabel(index: number, current: number): string {
  if (index < current) return 'completed';
  if (index === current) return 'current';
  return 'upcoming';
}

export function Stepper({ steps, current, orientation = 'horizontal', size = 'md' }: StepperProps) {
  return (
    <Steps.Root
      step={current}
      count={steps.length}
      orientation={orientation}
      size={size}
      colorPalette="emerald"
    >
      <Steps.List>
        {steps.map((step, index) => (
          <Steps.Item
            key={index}
            index={index}
            aria-current={index === current ? 'step' : undefined}
          >
            <Steps.Indicator>
              <Steps.Status
                complete={<Check size={16} aria-hidden />}
                incomplete={<Steps.Number />}
              />
            </Steps.Indicator>
            <Box>
              <Steps.Title>{step.title}</Steps.Title>
              {step.description ? <Steps.Description>{step.description}</Steps.Description> : null}
            </Box>
            <Box as="span" srOnly>
              {statusLabel(index, current)}
            </Box>
            {index < steps.length - 1 ? <Steps.Separator /> : null}
          </Steps.Item>
        ))}
      </Steps.List>
    </Steps.Root>
  );
}
