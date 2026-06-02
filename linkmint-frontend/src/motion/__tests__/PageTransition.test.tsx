import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { PageTransition } from '../PageTransition';

describe('PageTransition', () => {
  it('renders its children (content is never hidden behind the transition)', () => {
    renderWithTheme(
      <PageTransition>
        <p>Page body</p>
      </PageTransition>,
    );
    expect(screen.getByText('Page body')).toBeInTheDocument();
  });
});
