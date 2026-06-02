'use client';

/** Step 2 — presentational M-PESA pay instructions + best-effort payment-intent recording. */

import { Alert, Button, Card, Code, Heading, HStack, Stack, Text } from '@chakra-ui/react';
import { ArrowRight, Smartphone } from 'react-feather';
import { toast } from 'sonner';
import { useAppStore } from '@/store/app';
import { clientConfig } from '@/lib/env';
import { formatMinorUnits } from '@/lib/money';
import { useInitiatePayment } from '@/hooks/useInitiatePayment';
import { ErrorBanner } from './ErrorBanner';
import { KeyValueRow } from './KeyValueRow';

export function PayInstructions() {
  const data = useAppStore((s) => s.data);
  const proceed = useAppStore((s) => s.proceedToStatus);
  const paybill = clientConfig().mpesaPaybill;
  const initiate = useInitiatePayment(data?.plId ?? '');

  if (!data) return null;
  const amountText = formatMinorUnits(data.amount, data.currency);

  const copy = (label: string, value: string): void => {
    if (!navigator.clipboard) {
      toast.error('Clipboard unavailable');
      return;
    }
    void navigator.clipboard.writeText(value).then(() => toast.success(`${label} copied`));
  };

  return (
    <Card.Root>
      <Card.Header>
        <HStack gap={2}>
          <Smartphone size={20} />
          <Heading size="md">Pay with M-PESA</Heading>
        </HStack>
        <Text color="fg.muted" fontSize="sm" mt={1}>
          Complete the payment in your M-PESA app. This app never collects or holds funds.
        </Text>
      </Card.Header>
      <Card.Body>
        <Stack gap={4}>
          <Stack gap={2}>
            <KeyValueRow
              label="Pay Bill"
              value={paybill}
              onCopy={() => copy('Pay Bill', paybill)}
            />
            <KeyValueRow
              label="Account no."
              value={data.plId}
              mono
              onCopy={() => copy('Account number', data.plId)}
            />
            <KeyValueRow label="Amount" value={amountText} />
          </Stack>

          <Stack gap={1}>
            <Text fontWeight="medium">Steps</Text>
            <Text fontSize="sm">1. Open M-PESA → Lipa na M-PESA → Pay Bill.</Text>
            <Text fontSize="sm">
              2. Business no.: <Code>{paybill}</Code>.
            </Text>
            <Text fontSize="sm">3. Account no.: the PayLink id above.</Text>
            <Text fontSize="sm">
              4. Amount: <Code>{amountText}</Code> → confirm.
            </Text>
          </Stack>

          {initiate.status === 'not_payable' ? (
            <Alert.Root status="info" borderRadius="md">
              <Alert.Indicator />
              <Alert.Content>
                <Alert.Title>Payment intent not recorded (PAYLINK_NOT_PAYABLE)</Alert.Title>
                <Alert.Description>
                  Known work35 limitation — the PayLink is already PENDING on-chain. Settlement is
                  tracked from the PayLink itself on the next screen.
                </Alert.Description>
              </Alert.Content>
            </Alert.Root>
          ) : null}
          {initiate.status === 'recorded' ? (
            <Alert.Root status="success" borderRadius="md">
              <Alert.Indicator />
              <Alert.Content>
                <Alert.Title>Payment intent recorded</Alert.Title>
                <Alert.Description>
                  rail: mpesa · <Code>{initiate.payment.id}</Code>
                </Alert.Description>
              </Alert.Content>
            </Alert.Root>
          ) : null}
          {initiate.status === 'error' ? (
            <ErrorBanner error={initiate.error} status="warning" />
          ) : null}
        </Stack>
      </Card.Body>
      <Card.Footer>
        <Button colorPalette="emerald" onClick={proceed} gap={2} width="full">
          I’ve sent the payment — watch settlement <ArrowRight size={18} />
        </Button>
      </Card.Footer>
    </Card.Root>
  );
}
