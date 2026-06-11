import {screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import HomePage from './HomePage';

describe('HomePage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            id: 1,
            username: 'alice',
            email: 'a@b.c',
            totpEnabled: false,
          }),
          {status: 200},
        ),
      ),
    );
  });

  it('greets the logged-in user', async () => {
    renderWithProviders(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/welcome, alice/i)).toBeInTheDocument(),
    );
  });
});
