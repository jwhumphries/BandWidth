import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import Footer from './Footer';

describe('Footer', () => {
  it('links to the GitHub profile with the current year', () => {
    render(<Footer />);
    const link = screen.getByRole('link', {name: /john humphries/i});
    expect(link).toHaveAttribute('href', 'https://github.com/jwhumphries');
    expect(link).toHaveTextContent(String(new Date().getFullYear()));
  });
});
