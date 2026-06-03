import NextLink from 'next/link';
import { Link } from '@chakra-ui/react';

import { AuthLayout } from '@/components/auth/AuthLayout';
import { LoginForm } from '@/components/auth/LoginForm';

export const dynamic = 'force-dynamic';

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string }>;
}) {
  const { next } = await searchParams;
  return (
    <AuthLayout
      title="Welcome back"
      subtitle="Sign in to your LinkMint account."
      footer={
        <>
          New to LinkMint?{' '}
          <Link asChild color="accent.fg" fontWeight="500">
            <NextLink href="/register">Create an account</NextLink>
          </Link>
        </>
      }
    >
      <LoginForm next={typeof next === 'string' ? next : undefined} />
    </AuthLayout>
  );
}
