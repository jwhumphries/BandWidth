import {screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import Layout from './Layout';

describe('Layout', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith('/api/me')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                id: 1,
                username: 'jo',
                email: 'jo@x.com',
                totpEnabled: false,
                isAdmin: false,
              }),
              {status: 200},
            ),
          );
        }
        // /api/invites
        return Promise.resolve(new Response(JSON.stringify([]), {status: 200}));
      }),
    );
  });

  it('shows a Practice nav link pointing at /practice', () => {
    renderWithProviders(<Layout />);
    const link = screen.getByRole('link', {name: /practice/i});
    expect(link).toHaveAttribute('href', '/practice');
  });
});
