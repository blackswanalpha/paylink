'use client';

/**
 * Wizard shell. Builds the single SDK client from the server-minted dev token (client-side, so the
 * base URL resolves to the app origin) and stores it; then renders the active step.
 */

import { useEffect } from 'react';
import { Box, Center, Container, Heading, HStack, Spinner, Stack, Text } from '@chakra-ui/react';
import { Zap } from 'react-feather';
import { useAppStore } from '@/store/app';
import { createLinkMintClient } from '@/lib/linkmint';
import { CreatePayLinkForm } from './CreatePayLinkForm';
import { PayInstructions } from './PayInstructions';
import { SettlementStatus } from './SettlementStatus';

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
        </Box>

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
