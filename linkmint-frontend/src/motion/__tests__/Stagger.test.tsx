import { describe, it, expect } from 'vitest';
import { renderWithTheme, screen } from '@/test/renderWithTheme';
import { Stagger, StaggerItem } from '../Stagger';

describe('Stagger', () => {
  it('renders every item (stagger choreography never drops content)', () => {
    renderWithTheme(
      <Stagger>
        <StaggerItem>One</StaggerItem>
        <StaggerItem>Two</StaggerItem>
        <StaggerItem>Three</StaggerItem>
      </Stagger>,
    );
    expect(screen.getByText('One')).toBeInTheDocument();
    expect(screen.getByText('Two')).toBeInTheDocument();
    expect(screen.getByText('Three')).toBeInTheDocument();
  });
});
