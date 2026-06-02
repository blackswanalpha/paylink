'use client';

/**
 * Menu — an action menu wrapping Chakra v3 `Menu.*` (frontendfeature.md §2.5). Pass a `trigger`
 * element and a flat or grouped list of actions; selection reports the action's `value`. Used for
 * row "⋯" action menus (e.g. PayLink actions in work11).
 *
 * a11y (F.6): Chakra/Ark provide the full ARIA menu — `role="menu"/"menuitem"`, arrow-key navigation,
 * type-ahead, Home/End, Esc-to-close, focus return to the trigger, and `aria-haspopup`/`aria-expanded`
 * on the trigger. The trigger must carry an `aria-label`. Destructive items use the danger token plus
 * their label/icon, not color alone.
 */

import type { ReactNode } from 'react';
import { Box, Menu as MenuNamespace, Portal } from '@chakra-ui/react';

export interface MenuAction {
  value: string;
  label: ReactNode;
  icon?: ReactNode;
  /** @default 'default' */
  tone?: 'default' | 'danger';
  disabled?: boolean;
}

export interface MenuGroup {
  label?: ReactNode;
  items: MenuAction[];
}

export interface MenuProps {
  /** The trigger element (e.g. an IconButton with an aria-label). */
  trigger: ReactNode;
  /** Flat actions, or grouped sections. */
  items: MenuAction[] | MenuGroup[];
  onSelect: (value: string) => void;
  /** @default 'bottom-start' */
  placement?: 'bottom-start' | 'bottom-end' | 'top-start' | 'top-end';
}

function isGrouped(items: MenuAction[] | MenuGroup[]): items is MenuGroup[] {
  const first = items[0];
  return first !== undefined && 'items' in first;
}

function renderItem(action: MenuAction) {
  return (
    <MenuNamespace.Item
      key={action.value}
      value={action.value}
      disabled={action.disabled}
      color={action.tone === 'danger' ? 'status.danger' : undefined}
    >
      {action.icon ? (
        <Box as="span" display="inline-flex" me={2} aria-hidden>
          {action.icon}
        </Box>
      ) : null}
      {action.label}
    </MenuNamespace.Item>
  );
}

export function Menu({ trigger, items, onSelect, placement = 'bottom-start' }: MenuProps) {
  return (
    <MenuNamespace.Root onSelect={(details) => onSelect(details.value)} positioning={{ placement }}>
      <MenuNamespace.Trigger asChild>{trigger}</MenuNamespace.Trigger>
      <Portal>
        <MenuNamespace.Positioner>
          <MenuNamespace.Content
            bg="bg.panel"
            borderWidth="1px"
            borderColor="border"
            borderRadius="md"
            boxShadow="md"
            py={1}
            minW="180px"
          >
            {isGrouped(items)
              ? items.map((group, gi) => (
                  <MenuNamespace.ItemGroup key={gi}>
                    {group.label ? (
                      <MenuNamespace.ItemGroupLabel color="fg.muted" fontSize="xs">
                        {group.label}
                      </MenuNamespace.ItemGroupLabel>
                    ) : null}
                    {group.items.map(renderItem)}
                  </MenuNamespace.ItemGroup>
                ))
              : items.map(renderItem)}
          </MenuNamespace.Content>
        </MenuNamespace.Positioner>
      </Portal>
    </MenuNamespace.Root>
  );
}
