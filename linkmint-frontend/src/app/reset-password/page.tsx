import NextLink from 'next/link';
import { Link } from '@chakra-ui/react';

import { AuthLayout } from '@/components/auth/AuthLayout';
import { ResetPasswordForm } from '@/components/auth/ResetPasswordForm';

export const dynamic = 'force-dynamic';

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;
  return (
    <AuthLayout
      title="Choose a new password"
      subtitle="Set a new password to finish recovering your account."
      footer={
        <Link asChild color="accent.fg" fontWeight="500">
          <NextLink href="/login">Back to sign in</NextLink>
        </Link>
      }
    >
      <ResetPasswordForm token={typeof token === 'string' ? token : undefined} />
    </AuthLayout>
  );
}
