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

const bandSongs = [
  {
    id: 1,
    title: 'Aqualung',
    artist: 'Jethro Tull',
    status: 'learning',
    lastPracticedAt: '',
    practiceCount: 0,
  },
  {
    id: 2,
    title: 'Money',
    artist: 'Pink Floyd',
    status: 'learned',
    lastPracticedAt: '',
    practiceCount: 0,
  },
];

const bandFolders = [{id: 7, name: 'Set 1', position: 1, songIds: [2]}];

function renderBandPageWithFolders() {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/invites'))
        return Promise.resolve(jsonResponse(200, []));
      if (url.includes('/songs'))
        return Promise.resolve(jsonResponse(200, bandSongs));
      if (url.includes('/folders'))
        return Promise.resolve(jsonResponse(200, bandFolders));
      return Promise.resolve(jsonResponse(200, detail));
    }),
  );
  return renderWithProviders(
    <Routes>
      <Route path="/bands/:id" element={<BandPage />} />
    </Routes>,
    {route: '/bands/1'},
  );
}

describe('BandPage', () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    sessionStorage.clear();
  });

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

  it('restores the folder selected before visiting a song', async () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '7');
    renderBandPageWithFolders();

    // Only the stored folder's songs — not the whole band library.
    expect(await screen.findByText('Money')).toBeInTheDocument();
    expect(screen.queryByText('Aqualung')).not.toBeInTheDocument();
  });

  it('remembers the folder a member selects', async () => {
    renderBandPageWithFolders();
    await screen.findByText('Aqualung');

    await userEvent.click(screen.getByRole('button', {name: 'Set 1'}));

    await waitFor(() =>
      expect(sessionStorage.getItem('bandwidth-folder:band:1')).toBe('7'),
    );
  });

  it('ignores a stored folder that no longer exists', async () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '999');
    renderBandPageWithFolders();

    expect(await screen.findByText('Aqualung')).toBeInTheDocument();
    expect(screen.getByText('Money')).toBeInTheDocument();
    await waitFor(() =>
      expect(sessionStorage.getItem('bandwidth-folder:band:1')).toBeNull(),
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
