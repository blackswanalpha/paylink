import NextLink from 'next/link';
import { Link } from '@chakra-ui/react';

import { AuthLayout } from '@/components/auth/AuthLayout';
import { RegisterForm } from '@/components/auth/RegisterForm';

export const dynamic = 'force-dynamic';

export default function RegisterPage() {
  return (
    <AuthLayout
      title="Create your account"
      subtitle="Non-custodial by design — LinkMint never holds your funds."
      footer={
        <>
          Already have an account?{' '}
          <Link asChild color="accent.fg" fontWeight="500">
            <NextLink href="/login">Sign in</NextLink>
          </Link>
        </>
      }
    >
      <RegisterForm />
    </AuthLayout>
  );
}
