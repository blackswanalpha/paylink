'use client';

/**
 * Tabs — a config-driven wrapper over Chakra v3 `Tabs.*` (frontendfeature.md §2.5). Pass `items`
 * (value / label / icon / content) instead of hand-assembling triggers and panels; supports controlled
 * (`value` + `onChange`) or uncontrolled (`defaultValue`) use.
 *
 * a11y (F.6): Chakra/Ark provide the full ARIA tablist — roving tabindex, Arrow/Home/End keys,
 * `role="tab"/"tabpanel"`, `aria-selected`, and `aria-controls` wiring. No manual key handling needed.
 */

import type { ReactNode } from 'react';
import { Tabs as TabsNamespace } from '@chakra-ui/react';

export interface TabItem {
  value: string;
  label: ReactNode;
  icon?: ReactNode;
  content: ReactNode;
  disabled?: boolean;
}

export interface TabsProps {
  items: TabItem[];
  /** Controlled active value. Omit for uncontrolled. */
  value?: string;
  /** Uncontrolled initial value. @default the first item's value */
  defaultValue?: string;
  onChange?: (value: string) => void;
  /** @default 'line' */
  variant?: 'line' | 'enclosed' | 'subtle';
  /** @default 'md' */
  size?: 'sm' | 'md' | 'lg';
  /** Stretch triggers to fill the width. @default false */
  fitted?: boolean;
}

export function Tabs({
  items,
  value,
  defaultValue,
  onChange,
  variant = 'line',
  size = 'md',
  fitted = false,
}: TabsProps) {
  const resolvedDefault = defaultValue ?? items[0]?.value;
  return (
    <TabsNamespace.Root
      value={value}
      defaultValue={value === undefined ? resolvedDefault : undefined}
      onValueChange={(details) => onChange?.(details.value)}
      variant={variant}
      size={size}
      fitted={fitted}
      colorPalette="emerald"
    >
      <TabsNamespace.List>
        {items.map((item) => (
          <TabsNamespace.Trigger
            key={item.value}
            value={item.value}
            disabled={item.disabled}
            gap={2}
          >
            {item.icon}
            {item.label}
          </TabsNamespace.Trigger>
        ))}
      </TabsNamespace.List>
      {items.map((item) => (
        <TabsNamespace.Content key={item.value} value={item.value}>
          {item.content}
        </TabsNamespace.Content>
      ))}
    </TabsNamespace.Root>
  );
}
