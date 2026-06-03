'use client';

/**
 * AuthLayout — the centered card frame for the unauthenticated screens (login / register / MFA /
 * forgot-password). Ivory canvas, brand mark, Fraunces title + muted subtitle, the form, and an
 * optional footer slot (e.g. the cross-link to register/login). No sidebar/topbar — these screens
 * sit outside the dashboard shell.
 */

import type { ReactNode } from 'react';
import { Box, Flex, HStack, Stack, Text } from '@chakra-ui/react';

import { Panel } from '@/components/ui';

export interface AuthLayoutProps {
  title: string;
  subtitle?: string;
  children: ReactNode;
  footer?: ReactNode;
}

export function AuthLayout({ title, subtitle, children, footer }: AuthLayoutProps) {
  return (
    <Flex minH="100dvh" bg="canvas" align="center" justify="center" px={4} py={10}>
      <Box w="full" maxW="420px">
        <HStack gap={2} justify="center" mb={6}>
          <Text fontFamily="heading" fontSize="2xl" color="accent.solid" lineHeight="1">
            ◇
          </Text>
          <Text fontFamily="heading" fontWeight="600" fontSize="2xl" letterSpacing="-0.01em">
            LinkMint
          </Text>
        </HStack>

        <Panel as="section" p={{ base: 6, md: 8 }}>
          <Stack gap={1} mb={6}>
            <Text as="h1" fontFamily="heading" fontWeight="600" fontSize="xl">
              {title}
            </Text>
            {subtitle ? (
              <Text fontSize="sm" color="fg.muted">
                {subtitle}
              </Text>
            ) : null}
          </Stack>

          {children}
        </Panel>

        {footer ? (
          <Box mt={5} textAlign="center" fontSize="sm" color="fg.muted">
            {footer}
          </Box>
        ) : null}
      </Box>
    </Flex>
  );
}
