'use client';

/** Step 1 — controlled create-PayLink form with client-side validation. Calls the SDK via the hook. */

import { useMemo, useState, type FormEvent } from 'react';
import { Button, Card, Field, Heading, Input, Stack, Text } from '@chakra-ui/react';
import { PlusCircle } from 'react-feather';
import { useCreatePayLink } from '@/hooks/useCreatePayLink';
import { clientConfig } from '@/lib/env';
import { ErrorBanner } from './ErrorBanner';

const ADDRESS_RE = /^0x[0-9a-fA-F]{40}$/;

/** 24h from now, formatted for <input type="datetime-local"> (local time, minute precision). */
function defaultExpiryLocal(): string {
  const d = new Date(Date.now() + 24 * 60 * 60 * 1000);
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function CreatePayLinkForm() {
  const cfg = useMemo(() => clientConfig(), []);
  const { state, create } = useCreatePayLink();

  const [receiver, setReceiver] = useState(cfg.defaultReceiver);
  const [amount, setAmount] = useState('1000');
  const [currency, setCurrency] = useState(cfg.defaultCurrency);
  const [expiry, setExpiry] = useState(defaultExpiryLocal);
  const [touched, setTouched] = useState(false);

  const amountNum = Number(amount);
  const errors = {
    receiver: ADDRESS_RE.test(receiver) ? '' : 'Enter a 0x-prefixed 20-byte address.',
    amount:
      Number.isInteger(amountNum) && amountNum > 0 ? '' : 'Enter a positive integer (minor units).',
    currency: currency.trim().length > 0 ? '' : 'Required.',
    expiry: expiry && new Date(expiry).getTime() > Date.now() ? '' : 'Pick a future date/time.',
  };
  const valid = !errors.receiver && !errors.amount && !errors.currency && !errors.expiry;
  const loading = state.status === 'loading';
  const show = (msg: string): string | undefined => (touched && msg ? msg : undefined);

  const onSubmit = (e: FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    setTouched(true);
    if (!valid) return;
    void create({
      receiver: receiver.trim(),
      amount: amountNum,
      currency: currency.trim().toUpperCase(),
      expiry: new Date(expiry).toISOString(),
    });
  };

  return (
    <form onSubmit={onSubmit} noValidate>
      <Card.Root>
        <Card.Header>
          <Heading size="md">Create a PayLink</Heading>
          <Text color="fg.muted" fontSize="sm" mt={1}>
            An immutable authorization to receive a payment. Funds never touch this app.
          </Text>
        </Card.Header>
        <Card.Body>
          <Stack gap={4}>
            <Field.Root required invalid={Boolean(show(errors.receiver))}>
              <Field.Label>Receiver address</Field.Label>
              <Input
                value={receiver}
                onChange={(e) => setReceiver(e.target.value)}
                placeholder="0x1111111111111111111111111111111111111111"
                fontFamily="mono"
              />
              <Field.ErrorText>{show(errors.receiver)}</Field.ErrorText>
            </Field.Root>

            <Field.Root required invalid={Boolean(show(errors.amount))}>
              <Field.Label>Amount (minor units)</Field.Label>
              <Input
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                type="number"
                min={1}
                inputMode="numeric"
              />
              <Field.HelperText>e.g. 1000 = {currency || 'KES'} 10.00</Field.HelperText>
              <Field.ErrorText>{show(errors.amount)}</Field.ErrorText>
            </Field.Root>

            <Field.Root required invalid={Boolean(show(errors.currency))}>
              <Field.Label>Currency</Field.Label>
              <Input
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                maxLength={8}
                textTransform="uppercase"
              />
              <Field.ErrorText>{show(errors.currency)}</Field.ErrorText>
            </Field.Root>

            <Field.Root required invalid={Boolean(show(errors.expiry))}>
              <Field.Label>Expiry</Field.Label>
              <Input
                value={expiry}
                onChange={(e) => setExpiry(e.target.value)}
                type="datetime-local"
              />
              <Field.ErrorText>{show(errors.expiry)}</Field.ErrorText>
            </Field.Root>

            {state.status === 'error' ? <ErrorBanner error={state.error} /> : null}
          </Stack>
        </Card.Body>
        <Card.Footer>
          <Button
            type="submit"
            colorPalette="teal"
            loading={loading}
            loadingText="Creating…"
            gap={2}
            width="full"
          >
            <PlusCircle size={18} /> Create PayLink
          </Button>
        </Card.Footer>
      </Card.Root>
    </form>
  );
}
