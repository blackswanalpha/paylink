'use client';

/**
 * KitchenSink — a dev-only gallery rendering every work03 kit component in the Ivory light theme for
 * visual + keyboard verification (frontendfeature.md §2.5 / §7). This is a component catalogue, NOT a
 * product screen, so the data here is static fixtures and the F.7 "nothing faked" rule does not apply.
 * The route (app/kitchen-sink) 404s in production.
 */

import { useState, type ReactNode } from 'react';
import { Box, Container, Heading, HStack, Input, SimpleGrid, Stack, Text } from '@chakra-ui/react';
import { Eye, HelpCircle, MoreHorizontal, Trash2, User } from 'react-feather';
import {
  AddressChip,
  AmountDisplay,
  Avatar,
  Button,
  CopyField,
  DataTable,
  Drawer,
  EmptyState,
  FormField,
  GoldButton,
  IconButton,
  Menu,
  MetricCard,
  Modal,
  Panel,
  PayLinkStatusPill,
  QRBlock,
  Skeleton,
  Stepper,
  Tabs,
  Tooltip,
  type DataTableColumn,
} from '@/components/ui';
import { ErrorSystemDemo } from '@/components/dev/ErrorSystemDemo';

interface DemoRow {
  id: string;
  receiver: string;
  amount: number;
  currency: string;
}

const DEMO_ROWS: DemoRow[] = [
  {
    id: '0xpl_aaa',
    receiver: '0x1111111111111111111111111111111111111111',
    amount: 25000,
    currency: 'KES',
  },
  {
    id: '0xpl_bbb',
    receiver: '0x2222222222222222222222222222222222222222',
    amount: 12000,
    currency: 'KES',
  },
  {
    id: '0xpl_ccc',
    receiver: '0x3333333333333333333333333333333333333333',
    amount: 48000,
    currency: 'KES',
  },
];

const DEMO_COLUMNS: DataTableColumn<DemoRow>[] = [
  { key: 'id', header: 'PayLink', render: (r) => <AddressChip value={r.id} label="PayLink id" /> },
  {
    key: 'receiver',
    header: 'Receiver',
    hideBelow: 'md',
    render: (r) => <AddressChip value={r.receiver} label="receiver" />,
  },
  {
    key: 'amount',
    header: 'Amount',
    align: 'end',
    sortable: true,
    sortValue: (r) => r.amount,
    render: (r) => <AmountDisplay amountMinor={r.amount} currency={r.currency} size="sm" />,
  },
];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Panel>
      <Stack gap={4}>
        <Heading size="md" fontFamily="heading">
          {title}
        </Heading>
        {children}
      </Stack>
    </Panel>
  );
}

export function KitchenSink() {
  const [modalOpen, setModalOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [step, setStep] = useState(1);
  const [hasMore, setHasMore] = useState(true);

  return (
    <Box bg="canvas" minH="100dvh" py={10}>
      <Container maxW="4xl">
        <Stack gap={8}>
          <Box>
            <Heading size="xl" fontFamily="heading">
              Component kit
            </Heading>
            <Text color="fg.muted">
              work03 — every primitive in the Ivory Premium theme. Dev-only.
            </Text>
          </Box>

          <Section title="Buttons">
            <HStack gap={3} wrap="wrap">
              <Button colorPalette="emerald">Primary</Button>
              <Button variant="outline">Outline</Button>
              <Button variant="ghost">Ghost</Button>
              <GoldButton>Settled</GoldButton>
              <Button colorPalette="emerald" loading loadingText="Saving…">
                Save
              </Button>
            </HStack>
          </Section>

          <Section title="Tabs">
            <Tabs
              items={[
                { value: 'overview', label: 'Overview', content: <Text>Overview content.</Text> },
                { value: 'activity', label: 'Activity', content: <Text>Activity content.</Text> },
                { value: 'settings', label: 'Settings', content: <Text>Settings content.</Text> },
              ]}
            />
          </Section>

          <Section title="Form field">
            <Stack gap={4} maxW="sm">
              <FormField label="Receiver" helperText="A 0x-prefixed address" required>
                <Input placeholder="0x…" fontFamily="mono" />
              </FormField>
              <FormField label="Amount" error="Enter a positive integer.">
                <Input defaultValue="-5" />
              </FormField>
            </Stack>
          </Section>

          <Section title="Overlays & menu">
            <HStack gap={3}>
              <Button onClick={() => setModalOpen(true)}>Open modal</Button>
              <Button variant="outline" onClick={() => setDrawerOpen(true)}>
                Open drawer
              </Button>
              <Menu
                trigger={
                  <IconButton aria-label="Row actions" variant="outline">
                    <MoreHorizontal size={16} />
                  </IconButton>
                }
                items={[
                  { value: 'view', label: 'View', icon: <Eye size={14} /> },
                  { value: 'cancel', label: 'Cancel', icon: <Trash2 size={14} />, tone: 'danger' },
                ]}
                onSelect={() => undefined}
              />
            </HStack>
            <Modal
              open={modalOpen}
              onClose={() => setModalOpen(false)}
              title="Create PayLink"
              description="An immutable authorization to receive a payment."
              footer={
                <>
                  <Button variant="ghost" onClick={() => setModalOpen(false)}>
                    Cancel
                  </Button>
                  <Button colorPalette="emerald" onClick={() => setModalOpen(false)}>
                    Create
                  </Button>
                </>
              }
            >
              <Text>Modal body content.</Text>
            </Modal>
            <Drawer open={drawerOpen} onClose={() => setDrawerOpen(false)} title="PayLink detail">
              <Text>Drawer body content.</Text>
            </Drawer>
          </Section>

          <Section title="Stepper">
            <Stepper
              steps={[{ title: 'Create' }, { title: 'Instructions' }, { title: 'Settlement' }]}
              current={step}
            />
            <HStack gap={2}>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setStep((s) => Math.max(0, s - 1))}
              >
                Back
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setStep((s) => Math.min(2, s + 1))}
              >
                Next
              </Button>
            </HStack>
          </Section>

          <Section title="Avatars & tooltip">
            <HStack gap={4}>
              <Avatar name="Mary Wanjiku" />
              <Avatar name="LinkMint Merchant" size="lg" />
              <Avatar icon={<User size={18} />} />
              <Tooltip content="Non-custodial — funds never touch LinkMint">
                <IconButton aria-label="Help" variant="ghost">
                  <HelpCircle size={16} />
                </IconButton>
              </Tooltip>
            </HStack>
          </Section>

          <Section title="Copy & QR">
            <Stack gap={4} maxW="md">
              <CopyField label="PayLink id" value="0xpl_aaa111bbb222" mono />
              <CopyField
                label="address"
                value="0x1111111111111111111111111111111111111111"
                mono
                variant="inline"
                truncate={{ head: 6, tail: 4 }}
              />
              <QRBlock
                value="https://pay.linkmint.example/0xpl_aaa"
                label="QR code for this PayLink"
                downloadable
                caption={
                  <CopyField label="PayLink URL" value="https://pay.linkmint.example/0xpl_aaa" />
                }
              />
            </Stack>
          </Section>

          <Section title="Status, amounts, metrics">
            <HStack gap={3} wrap="wrap">
              <PayLinkStatusPill status="VERIFIED" />
              <PayLinkStatusPill status="PENDING" />
              <PayLinkStatusPill status="FAILED" />
              <PayLinkStatusPill status="EXPIRED" />
              <AmountDisplay amountMinor={125000} currency="KES" />
            </HStack>
            <SimpleGrid columns={{ base: 1, sm: 3 }} gap={4}>
              <MetricCard
                label="Total settled"
                value={<AmountDisplay amountMinor={480000} currency="KES" size="md" />}
                sparkline={[3, 5, 4, 8, 6, 9, 7, 11]}
              />
              <MetricCard label="Active" value="12" />
              <MetricCard label="Pending" value="3" />
            </SimpleGrid>
          </Section>

          <Section title="DataTable">
            <DataTable
              columns={DEMO_COLUMNS}
              rows={DEMO_ROWS}
              rowKey={(r) => r.id}
              caption="Demo PayLinks"
              interactive
              hasMore={hasMore}
              onLoadMore={() => setHasMore(false)}
            />
          </Section>

          <Section title="Empty & loading">
            <SimpleGrid columns={{ base: 1, md: 2 }} gap={4}>
              <EmptyState
                title="No PayLinks yet"
                description="Create your first PayLink to see it here."
              />
              <Stack gap={3}>
                <Skeleton height="1.5rem" />
                <Skeleton height="1.5rem" />
                <Skeleton height="1.5rem" width="60%" />
              </Stack>
            </SimpleGrid>
          </Section>

          <Section title="Error & feedback system (work04)">
            <ErrorSystemDemo />
          </Section>
        </Stack>
      </Container>
    </Box>
  );
}
