/**
 * Smoke tests for the work03 component kit: render each component under the real Ivory theme and
 * assert (a) it doesn't crash and (b) its key accessibility contract holds (roles, aria-*). These
 * guard the kit's a11y promises (F.6) without asserting pixel styling. Portalled components
 * (Modal/Drawer/Tooltip/Menu) render into document.body, so we query via `screen`/`findBy*`.
 */

import { describe, it, expect, vi } from 'vitest';
import { Input } from '@chakra-ui/react';
import { Plus } from 'react-feather';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { Button, GoldButton, IconButton } from '../Button';
import { Modal } from '../Modal';
import { Drawer } from '../Drawer';
import { Tabs } from '../Tabs';
import { Stepper } from '../Stepper';
import { FormField } from '../FormField';
import { Tooltip } from '../Tooltip';
import { Menu } from '../Menu';
import { Avatar } from '../Avatar';
import { CopyField } from '../CopyField';
import { QRBlock } from '../QRBlock';
import { DataTable, type DataTableColumn } from '../DataTable';

describe('Button', () => {
  it('renders with its label', () => {
    renderWithTheme(<Button>Save</Button>);
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('shows the loading text while loading', () => {
    renderWithTheme(
      <Button loading loadingText="Saving…">
        Save
      </Button>,
    );
    expect(screen.getByRole('button', { name: 'Saving…' })).toBeInTheDocument();
  });

  it('GoldButton renders a celebratory button', () => {
    renderWithTheme(<GoldButton>Settled</GoldButton>);
    expect(screen.getByRole('button', { name: 'Settled' })).toBeInTheDocument();
  });

  it('IconButton exposes its aria-label', () => {
    renderWithTheme(
      <IconButton aria-label="Add item">
        <Plus size={16} />
      </IconButton>,
    );
    expect(screen.getByRole('button', { name: 'Add item' })).toBeInTheDocument();
  });
});

describe('Modal', () => {
  it('renders an accessible, labelled dialog with a close button', () => {
    renderWithTheme(
      <Modal open onClose={() => undefined} title="Confirm action">
        Body content
      </Modal>,
    );
    expect(screen.getByRole('dialog', { name: 'Confirm action' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close dialog' })).toBeInTheDocument();
  });

  it('calls onClose when Escape is pressed', async () => {
    const onClose = vi.fn();
    const { user } = renderWithTheme(
      <Modal open onClose={onClose} title="Confirm">
        Body
      </Modal>,
    );
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('Drawer', () => {
  it('renders an accessible, labelled panel with a close button', () => {
    renderWithTheme(
      <Drawer open onClose={() => undefined} title="PayLink detail">
        Body content
      </Drawer>,
    );
    expect(screen.getByRole('dialog', { name: 'PayLink detail' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close panel' })).toBeInTheDocument();
  });
});

describe('Tabs', () => {
  const items = [
    { value: 'overview', label: 'Overview', content: <p>Overview panel</p> },
    { value: 'activity', label: 'Activity', content: <p>Activity panel</p> },
  ];

  it('renders a tablist, selects the first tab, and switches panel on activation', async () => {
    const { user } = renderWithTheme(<Tabs items={items} />);
    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(2);
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Overview panel');
    await user.click(screen.getByRole('tab', { name: 'Activity' }));
    expect(screen.getByRole('tab', { name: 'Activity' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Activity panel');
  });
});

describe('Stepper', () => {
  const steps = [{ title: 'Create' }, { title: 'Instructions' }, { title: 'Settlement' }];

  it('marks the current step and conveys state without color', () => {
    const { container } = renderWithTheme(<Stepper steps={steps} current={1} />);
    expect(screen.getByText('Create')).toBeInTheDocument();
    expect(container.querySelector('[aria-current="step"]')).toBeInTheDocument();
    // Visually-hidden per-step status (non-color cue): step 0 completed, step 1 current.
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('current')).toBeInTheDocument();
    expect(screen.getByText('upcoming')).toBeInTheDocument();
  });
});

describe('FormField', () => {
  it('associates the label with the control and shows helper text', () => {
    renderWithTheme(
      <FormField label="Receiver" helperText="A 0x address">
        <Input />
      </FormField>,
    );
    const input = screen.getByLabelText('Receiver');
    expect(input).toBeInTheDocument();
    expect(input).not.toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText('A 0x address')).toBeInTheDocument();
  });

  it('renders invalid and shows the error in place of the helper', () => {
    renderWithTheme(
      <FormField label="Amount" helperText="Minor units" error="Enter a positive integer.">
        <Input />
      </FormField>,
    );
    expect(screen.getByLabelText('Amount')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText('Enter a positive integer.')).toBeInTheDocument();
    expect(screen.queryByText('Minor units')).not.toBeInTheDocument();
  });
});

describe('Tooltip', () => {
  it('reveals its content on keyboard focus', async () => {
    const { user } = renderWithTheme(
      <Tooltip content="Copy to clipboard">
        <button type="button">Copy</button>
      </Tooltip>,
    );
    await user.tab();
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Copy to clipboard');
  });
});

describe('Menu', () => {
  it('opens from its trigger and reports the selected action value', async () => {
    const onSelect = vi.fn();
    const { user } = renderWithTheme(
      <Menu
        trigger={<button type="button">Actions</button>}
        items={[
          { value: 'view', label: 'View' },
          { value: 'cancel', label: 'Cancel', tone: 'danger' },
        ]}
        onSelect={onSelect}
      />,
    );
    const trigger = screen.getByRole('button', { name: 'Actions' });
    expect(trigger).toHaveAttribute('aria-haspopup');
    await user.click(trigger);
    expect(await screen.findAllByRole('menuitem')).toHaveLength(2);
    await user.click(screen.getByRole('menuitem', { name: 'View' }));
    expect(onSelect).toHaveBeenCalledWith('view');
  });
});

describe('Avatar', () => {
  it('shows initials derived from the name', () => {
    renderWithTheme(<Avatar name="Mary Wanjiku" />);
    expect(screen.getByText('MW')).toBeInTheDocument();
  });
});

describe('CopyField', () => {
  it('copies the full value even when the display is truncated', async () => {
    const { user } = renderWithTheme(
      <CopyField value="0x1234567890" label="address" mono truncate={{ head: 4, tail: 4 }} />,
    );
    // Spy on the live clipboard (user-event installs its own stub during setup()).
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    expect(screen.getByText('0x12…7890')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Copy address' }));
    expect(writeText).toHaveBeenCalledWith('0x1234567890');
  });
});

describe('QRBlock', () => {
  it('exposes the QR as a labelled image', () => {
    renderWithTheme(<QRBlock value="https://pay.linkmint/abc" label="PayLink QR code" />);
    expect(screen.getByRole('img', { name: 'PayLink QR code' })).toBeInTheDocument();
  });
});

describe('DataTable', () => {
  interface Row {
    id: string;
    name: string;
    amount: number;
  }
  const rows: Row[] = [
    { id: 'a', name: 'Bravo', amount: 30 },
    { id: 'b', name: 'Alpha', amount: 10 },
    { id: 'c', name: 'Charlie', amount: 20 },
  ];
  const columns: DataTableColumn<Row>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (r) => r.name,
      sortable: true,
      sortValue: (r) => r.name,
    },
    { key: 'amount', header: 'Amount', align: 'end', render: (r) => r.amount },
  ];

  it('renders a captioned table; sortable headers expose aria-sort and toggle on click', async () => {
    const { user } = renderWithTheme(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} caption="PayLinks" />,
    );
    expect(screen.getByRole('table', { name: 'PayLinks' })).toBeInTheDocument();
    const headers = screen.getAllByRole('columnheader');
    expect(headers[0]).toHaveAttribute('aria-sort', 'none'); // sortable, inactive
    expect(headers[1]).not.toHaveAttribute('aria-sort'); // not sortable → omitted
    await user.click(screen.getByRole('button', { name: /Name/ }));
    expect(screen.getAllByRole('columnheader')[0]).toHaveAttribute('aria-sort', 'ascending');
  });

  it('renders the empty node when there are no rows', () => {
    renderWithTheme(
      <DataTable
        columns={columns}
        rows={[]}
        rowKey={(r) => r.id}
        empty={<span>No PayLinks</span>}
      />,
    );
    expect(screen.getByText('No PayLinks')).toBeInTheDocument();
  });

  it('shows a Load more button that fires onLoadMore', async () => {
    const onLoadMore = vi.fn();
    const { user } = renderWithTheme(
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => r.id}
        hasMore
        onLoadMore={onLoadMore}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Load more' }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });
});
