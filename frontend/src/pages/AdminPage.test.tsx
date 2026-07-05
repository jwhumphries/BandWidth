import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import AdminPage from './AdminPage';

const users = [
  {
    id: 1,
    username: 'admin',
    email: 'admin@example.com',
    createdAt: '2026-01-01',
  },
  {id: 2, username: 'bob', email: 'bob@example.com', createdAt: '2026-01-02'},
];
const bands = [
  {id: 1, name: 'The Quietones', creatorUsername: 'admin', memberCount: 2},
];
let policy: {
  enabled: boolean;
  allowedEmails: Array<{id: number; email: string; createdAt: string}>;
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('AdminPage', () => {
  beforeEach(() => {
    policy = {enabled: false, allowedEmails: []};
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          const method = init?.method ?? 'GET';
          if (url.endsWith('/api/admin/users') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, users));
          }
          if (url.includes('/api/admin/users/') && method === 'DELETE') {
            return Promise.resolve(jsonResponse(204, null));
          }
          if (url.endsWith('/api/admin/bands') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, bands));
          }
          if (url.includes('/api/admin/bands/') && method === 'DELETE') {
            return Promise.resolve(jsonResponse(204, null));
          }
          if (url.endsWith('/api/admin/access-policy') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, policy));
          }
          if (url.endsWith('/api/admin/access-policy') && method === 'PUT') {
            return Promise.resolve(jsonResponse(204, null));
          }
          if (
            url.endsWith('/api/admin/access-policy/emails') &&
            method === 'POST'
          ) {
            return Promise.resolve(
              jsonResponse(201, {id: 9, email: 'friend@example.com'}),
            );
          }
          if (
            url.includes('/api/admin/access-policy/emails/') &&
            method === 'DELETE'
          ) {
            return Promise.resolve(jsonResponse(204, null));
          }
          return Promise.resolve(jsonResponse(404, {message: 'not found'}));
        }),
    );
  });

  it('lists users and deletes one after confirming', async () => {
    renderWithProviders(<AdminPage />);
    expect(await screen.findByText('bob')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: /delete bob/i}));
    await userEvent.click(await screen.findByRole('button', {name: 'Delete'}));

    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/users/2') &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });

  it('lists bands on the Bands tab', async () => {
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /bands/i}));
    expect(await screen.findByText('The Quietones')).toBeInTheDocument();
    expect(screen.getByText(/created by admin/i)).toBeInTheDocument();
  });

  it('deletes a band after confirming', async () => {
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /bands/i}));
    await screen.findByText('The Quietones');

    await userEvent.click(
      screen.getByRole('button', {name: /delete the quietones/i}),
    );
    await userEvent.click(await screen.findByRole('button', {name: 'Delete'}));

    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/bands/1') &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });

  it('toggles the access policy and adds an allowed email', async () => {
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /access policy/i}));
    await screen.findByText(/registration open to anyone/i);

    await userEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/access-policy') &&
            init?.method === 'PUT' &&
            String(init.body).includes('true'),
        ),
      ).toBe(true);
    });

    await userEvent.type(
      screen.getByPlaceholderText(/friend@example.com/i),
      'friend@example.com',
    );
    await userEvent.click(screen.getByRole('button', {name: /add/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/access-policy/emails') &&
            init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });

  it('removes an allowed email', async () => {
    policy = {
      enabled: true,
      allowedEmails: [
        {id: 5, email: 'friend@example.com', createdAt: '2026-01-01'},
      ],
    };
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /access policy/i}));

    await userEvent.click(
      screen.getByRole('button', {name: /remove friend@example.com/i}),
    );

    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/access-policy/emails/5') &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });
});
