import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import BandPage from './BandPage';

const detail = {
  id: 1,
  name: 'The Quietones',
  creatorId: 10,
  myRole: 'admin',
  members: [
    {userId: 10, username: 'alice', role: 'admin'},
    {userId: 11, username: 'bob', role: 'editor'},
  ],
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

function renderBandPage(myRole = 'admin') {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/invites') && init?.method === 'POST') {
          return Promise.resolve(
            jsonResponse(201, {role: 'viewer', token: 'TOK123', isLink: true}),
          );
        }
        if (url.includes('/invites')) {
          return Promise.resolve(jsonResponse(200, []));
        }
        if (url.includes('/songs')) {
          return Promise.resolve(jsonResponse(200, []));
        }
        if (url.includes('/folders')) {
          return Promise.resolve(jsonResponse(200, []));
        }
        if (init?.method === 'PATCH' || init?.method === 'DELETE') {
          return Promise.resolve(new Response(null, {status: 204}));
        }
        return Promise.resolve(jsonResponse(200, {...detail, myRole}));
      }),
  );
  return renderWithProviders(
    <Routes>
      <Route path="/bands/:id" element={<BandPage />} />
      <Route path="/bands" element={<p>bands list</p>} />
    </Routes>,
    {route: '/bands/1'},
  );
}

describe('BandPage', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('shows the roster with roles', async () => {
    renderBandPage();
    expect(await screen.findByText('alice')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
  });

  it('admins can change a member role', async () => {
    renderBandPage('admin');
    await screen.findByText('bob');
    const select = screen.getByLabelText(/role for bob/i);
    await userEvent.selectOptions(select, 'viewer');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/members/11') && init?.method === 'PATCH',
        ),
      ).toBe(true);
    });
  });

  it('admins can create a share link and see the token once', async () => {
    renderBandPage('admin');
    await screen.findByText('bob');
    await userEvent.click(
      screen.getByRole('button', {name: /create invite link/i}),
    );
    await waitFor(() =>
      expect(screen.getByText(/\/join\/TOK123/)).toBeInTheDocument(),
    );
  });

  it('non-admins see the roster but no management controls', async () => {
    renderBandPage('viewer');
    await screen.findByText('bob');
    expect(screen.queryByLabelText(/role for bob/i)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /create invite link/i}),
    ).not.toBeInTheDocument();
  });
});
