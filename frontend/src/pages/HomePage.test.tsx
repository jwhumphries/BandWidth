import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import HomePage from './HomePage';

const songs = [
  {
    id: 1,
    title: 'Wonderwall',
    artist: 'Oasis',
    status: 'learning',
    lastPracticedAt: '2026-06-10',
    practiceCount: 3,
  },
  {
    id: 2,
    title: 'Creep',
    artist: 'Radiohead',
    status: 'nailed',
    lastPracticedAt: '',
    practiceCount: 0,
  },
];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('HomePage library', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          if (url.includes('/api/songs') && init?.method === 'PUT') {
            return Promise.resolve(
              jsonResponse(200, {
                lastPracticedAt: '2026-06-11',
                practiceCount: 4,
              }),
            );
          }
          if (url.includes('/api/songs') && init?.method === 'POST') {
            return Promise.resolve(
              jsonResponse(201, {...songs[0], id: 3, title: 'New One'}),
            );
          }
          if (url.includes('/api/folders')) {
            return Promise.resolve(jsonResponse(200, []));
          }
          if (url.includes('/api/songs')) {
            return Promise.resolve(jsonResponse(200, songs));
          }
          return Promise.resolve(
            jsonResponse(200, {
              id: 1,
              username: 'alice',
              email: 'a@b.c',
              totpEnabled: false,
            }),
          );
        }),
    );
  });

  it('lists songs with status badges', async () => {
    renderWithProviders(<HomePage />);
    expect(await screen.findByText('Wonderwall')).toBeInTheDocument();
    expect(screen.getByText('Creep')).toBeInTheDocument();
    expect(screen.getByText(/nailed!/i)).toBeInTheDocument();
  });

  it('filters with the search box', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    await userEvent.type(screen.getByPlaceholderText(/search/i), 'creep');
    await waitFor(() =>
      expect(screen.queryByText('Wonderwall')).not.toBeInTheDocument(),
    );
    expect(screen.getByText('Creep')).toBeInTheDocument();
  });

  it('logs practice and offers undo', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    const buttons = screen.getAllByRole('button', {name: /practiced/i});
    await userEvent.click(buttons[0]!);
    await waitFor(() =>
      expect(screen.getByRole('button', {name: /undo/i})).toBeInTheDocument(),
    );
  });

  it('orders a folder alphabetically with no reorder handles', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/folders')) {
          return Promise.resolve(
            jsonResponse(200, [
              {id: 4, name: 'Setlist', position: 1, songIds: [1, 2]},
            ]),
          );
        }
        if (url.includes('/api/songs')) {
          return Promise.resolve(jsonResponse(200, songs));
        }
        return Promise.resolve(jsonResponse(200, {}));
      }),
    );

    renderWithProviders(<HomePage />);
    await userEvent.click(await screen.findByRole('button', {name: 'Setlist'}));

    const links = await screen.findAllByRole('link');
    expect(links.map(l => l.textContent)).toEqual([
      'CreepRadiohead',
      'WonderwallOasis',
    ]);
    expect(
      screen.queryByRole('button', {name: /reorder wonderwall/i}),
    ).not.toBeInTheDocument();
  });

  it('restores the folder selected before visiting a song', async () => {
    sessionStorage.setItem('bandwidth-folder:personal', '4');
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/folders')) {
          return Promise.resolve(
            jsonResponse(200, [
              {id: 4, name: 'Setlist', position: 1, songIds: [2]},
            ]),
          );
        }
        if (url.includes('/api/songs')) {
          return Promise.resolve(jsonResponse(200, songs));
        }
        return Promise.resolve(jsonResponse(200, {}));
      }),
    );

    renderWithProviders(<HomePage />);

    expect(
      await screen.findByRole('heading', {name: 'Setlist'}),
    ).toBeInTheDocument();
    expect(screen.getByText('Creep')).toBeInTheDocument();
    expect(screen.queryByText('Wonderwall')).not.toBeInTheDocument();
  });

  it('adds a song through the modal', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    await userEvent.click(screen.getByRole('button', {name: /add song/i}));
    await userEvent.type(screen.getByLabelText(/title/i), 'New One');
    await userEvent.click(screen.getByRole('button', {name: /^add$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([, init]) =>
            init?.method === 'POST' && String(init.body).includes('New One'),
        ),
      ).toBe(true);
    });
  });
});
