import {render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import HomePage from './HomePage';

describe('HomePage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ok: true} as unknown as Response),
    );
  });

  it('renders the app name', () => {
    render(<HomePage />);
    expect(
      screen.getByRole('heading', {name: /bandwidth/i}),
    ).toBeInTheDocument();
  });

  it('shows server online once the health check resolves', async () => {
    render(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/server online/i)).toBeInTheDocument(),
    );
  });

  it('shows server unreachable when the health check fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    render(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/server unreachable/i)).toBeInTheDocument(),
    );
  });
});
