import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import JoinPage from './JoinPage';

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

// loggedIn controls how /api/me resolves.
function mockFetch(loggedIn: boolean) {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (
          url.includes('/api/invites/link/') &&
          (!init || init.method === undefined)
        ) {
          return Promise.resolve(json(200, {bandName: 'The Quietones'}));
        }
        if (url.endsWith('/api/me')) {
          return loggedIn
            ? Promise.resolve(
                json(200, {
                  id: 1,
                  username: 'a',
                  email: 'a@b.c',
                  totpEnabled: false,
                }),
              )
            : Promise.resolve(json(401, {message: 'authentication required'}));
        }
        if (url.includes('/api/invites/link') && init?.method === 'POST') {
          return Promise.resolve(json(200, {bandId: 7}));
        }
        return Promise.resolve(json(404, {message: 'not found'}));
      }),
  );
}

describe('JoinPage', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('shows the band name and log in / sign up when logged out', async () => {
    mockFetch(false);
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/abc'},
    );
    expect(await screen.findByText(/The Quietones/)).toBeInTheDocument();
    const login = screen.getByRole('link', {name: /log in/i});
    expect(login).toHaveAttribute('href', '/login?redirect=%2Fjoin%2Fabc');
    expect(screen.getByRole('link', {name: /sign up/i})).toHaveAttribute(
      'href',
      '/signup?redirect=%2Fjoin%2Fabc',
    );
  });

  it('joins and navigates to the band when logged in', async () => {
    mockFetch(true);
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
        <Route path="/bands/7" element={<p>band page</p>} />
      </Routes>,
      {route: '/join/abc'},
    );
    await userEvent.click(await screen.findByRole('button', {name: /join/i}));
    await waitFor(() =>
      expect(screen.getByText('band page')).toBeInTheDocument(),
    );
  });

  it('shows an error for an invalid token', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(json(404, {message: 'invite not found'})),
    );
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/bad'},
    );
    expect(
      await screen.findByText(/invalid or has expired/i),
    ).toBeInTheDocument();
  });

  it('offers a retry instead of "invalid" when the preview fails transiently', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(json(429, {message: 'rate limit exceeded'})),
    );
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/abc'},
    );
    expect(
      await screen.findByText(/could not load this invite/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/invalid or has expired/i),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /retry/i})).toBeInTheDocument();
  });

  it('loads the invite when retry is clicked after a transient failure', async () => {
    let previewCalls = 0;
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          if (
            url.includes('/api/invites/link/') &&
            (!init || init.method === undefined)
          ) {
            previewCalls++;
            return previewCalls === 1
              ? Promise.resolve(json(500, {message: 'internal error'}))
              : Promise.resolve(json(200, {bandName: 'The Quietones'}));
          }
          if (url.endsWith('/api/me')) {
            return Promise.resolve(
              json(401, {message: 'authentication required'}),
            );
          }
          return Promise.resolve(json(404, {message: 'not found'}));
        }),
    );
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/abc'},
    );
    await userEvent.click(await screen.findByRole('button', {name: /retry/i}));
    expect(await screen.findByText(/The Quietones/)).toBeInTheDocument();
  });

  it('shows a server error instead of log in links when /api/me fails transiently', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          if (
            url.includes('/api/invites/link/') &&
            (!init || init.method === undefined)
          ) {
            return Promise.resolve(json(200, {bandName: 'The Quietones'}));
          }
          if (url.endsWith('/api/me')) {
            return Promise.resolve(json(500, {message: 'internal error'}));
          }
          return Promise.resolve(json(404, {message: 'not found'}));
        }),
    );
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/abc'},
    );
    expect(
      await screen.findByText(/could not reach the server/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('link', {name: /log in/i}),
    ).not.toBeInTheDocument();
  });
});
