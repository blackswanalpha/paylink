'use client';

/**
 * Wizard shell. Builds the single SDK client from the server-minted dev token (client-side, so the
 * base URL resolves to the app origin) and stores it; then renders the active step.
 */

import { useEffect } from 'react';
import NextLink from 'next/link';
import {
  Box,
  Center,
  Container,
  Heading,
  HStack,
  Link,
  Spinner,
  Stack,
  Text,
} from '@chakra-ui/react';
import { ArrowRight, Zap } from 'react-feather';
import { useAppStore } from '@/store/app';
import { createLinkMintClient } from '@/lib/linkmint';
import { Stepper } from '@/components/ui';
import type { Step } from '@/types/wizard';
import { CreatePayLinkForm } from './CreatePayLinkForm';
import { PayInstructions } from './PayInstructions';
import { SettlementStatus } from './SettlementStatus';

const WIZARD_STEPS = [{ title: 'Create' }, { title: 'Pay' }, { title: 'Settlement' }];
const STEP_INDEX: Record<Step, number> = { create: 0, instructions: 1, status: 2 };

export function PayLinkDemo({ initialToken }: { initialToken: string }) {
  const client = useAppStore((s) => s.client);
  const setClient = useAppStore((s) => s.setClient);
  const step = useAppStore((s) => s.step);

  useEffect(() => {
    setClient(createLinkMintClient(initialToken));
  }, [initialToken, setClient]);

  return (
    <Container maxW="lg" py={{ base: 8, md: 12 }}>
      <Stack gap={6}>
        <Box>
          <HStack gap={2}>
            <Zap size={22} />
            <Heading size="lg">LinkMint</Heading>
          </HStack>
          <Text color="fg.muted" mt={1}>
            Create a PayLink → pay via M-PESA → watch it settle on-chain.
          </Text>
          <Link
            asChild
            mt={2}
            fontSize="sm"
            fontWeight="500"
            color="accent.fg"
            display="inline-flex"
            alignItems="center"
            gap={1}
          >
            <NextLink href="/dashboard">
              Go to merchant dashboard <ArrowRight size={14} />
            </NextLink>
          </Link>
        </Box>

        <Stepper steps={WIZARD_STEPS} current={STEP_INDEX[step]} />

        {!client ? (
          <Center py={16}>
            <Spinner size="lg" />
          </Center>
        ) : step === 'create' ? (
          <CreatePayLinkForm />
        ) : step === 'instructions' ? (
          <PayInstructions />
        ) : (
          <SettlementStatus />
        )}
      </Stack>
    </Container>
  );
}
