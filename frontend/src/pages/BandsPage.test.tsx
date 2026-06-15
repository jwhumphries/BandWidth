import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import BandsPage from './BandsPage';

const bands = [{id: 1, name: 'The Quietones', role: 'admin', memberCount: 3}];
const invites = [{id: 9, bandId: 2, bandName: 'Loud Ones', role: 'editor'}];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandsPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          if (url.includes('/api/invites') && init?.method === 'POST') {
            return Promise.resolve(jsonResponse(200, {bandId: 2}));
          }
          if (url.includes('/api/invites')) {
            return Promise.resolve(jsonResponse(200, invites));
          }
          if (url.includes('/api/bands') && init?.method === 'POST') {
            return Promise.resolve(jsonResponse(201, {id: 5}));
          }
          return Promise.resolve(jsonResponse(200, bands));
        }),
    );
  });

  it('lists bands with role and member count', async () => {
    renderWithProviders(<BandsPage />);
    expect(await screen.findByText('The Quietones')).toBeInTheDocument();
    expect(screen.getByText(/admin/i)).toBeInTheDocument();
    expect(screen.getByText(/3 members/i)).toBeInTheDocument();
  });

  it('shows pending invites with accept/decline', async () => {
    renderWithProviders(<BandsPage />);
    expect(await screen.findByText('Loud Ones')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: /accept/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/accept') && init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });

  it('creates a band', async () => {
    renderWithProviders(<BandsPage />);
    await screen.findByText('The Quietones');
    await userEvent.type(
      screen.getByPlaceholderText(/new band/i),
      'Fresh Band',
    );
    await userEvent.click(screen.getByRole('button', {name: /create/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands') &&
            init?.method === 'POST' &&
            String(init.body).includes('Fresh Band'),
        ),
      ).toBe(true);
    });
  });
});
