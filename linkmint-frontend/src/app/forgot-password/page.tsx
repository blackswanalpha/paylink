import NextLink from 'next/link';
import { Link } from '@chakra-ui/react';

import { AuthLayout } from '@/components/auth/AuthLayout';
import { ForgotPasswordForm } from '@/components/auth/ForgotPasswordForm';

export const dynamic = 'force-dynamic';

export default function ForgotPasswordPage() {
  return (
    <AuthLayout
      title="Reset your password"
      subtitle="We'll email you a link to set a new password."
      footer={
        <Link asChild color="accent.fg" fontWeight="500">
          <NextLink href="/login">Back to sign in</NextLink>
        </Link>
      }
    >
      <ForgotPasswordForm />
    </AuthLayout>
  );
}
