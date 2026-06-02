# flow23 — Command Palette & Global Search (execution recipe)

**Work item:** [work23](../work/work23.md) · **Goal recap:** a ⌘K palette for navigation, quick actions, and id lookups.

## Pre-flight
- [ ] Read [work23](../work/work23.md), [frontendfeature.md §1](../../../frontendfeature.md), the `nav.ts` model.
- [ ] Confirm work02 (+ Modal from work03) are usable.
- [ ] Set work23 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map nav commands + SDK lookups + recent-items source | **Explore** | command set |
| 2 | Design the palette (groups, keyboard model, find-by-id) | **Plan** | short design |
| 3 | Implement the ⌘K palette (modal, fuzzy list, actions, lookups) | **service-builder** | palette |
| 4 | Keyboard model + recents + theme toggle; tests | **service-builder** | passing |
| 5 | Review F.6 keyboard/focus | `/code-review` | clean diff |
| 6 | ⌘K navigate/create/find against live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work23](../work/work23.md) met; **App** checklist complete; status `done`.
