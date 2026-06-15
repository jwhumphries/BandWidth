import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import BandSongList from './BandSongList';

const songs = [
  {
    id: 1,
    title: 'Wonderwall',
    artist: 'Oasis',
    status: 'learned',
    lastPracticedAt: '',
    practiceCount: 0,
  },
];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandSongList', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          if (init?.method === 'POST') {
            return Promise.resolve(jsonResponse(201, {id: 9}));
          }
          return Promise.resolve(jsonResponse(200, songs));
        }),
    );
  });

  it('lists band songs linking to the band-song detail', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={false} />);
    const link = await screen.findByRole('link', {name: /wonderwall/i});
    expect(link).toHaveAttribute('href', '/bands/3/songs/1');
    // Viewers get no add control.
    expect(
      screen.queryByRole('button', {name: /add song/i}),
    ).not.toBeInTheDocument();
  });

  it('lets editors add a band song', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={true} />);
    await screen.findByRole('link', {name: /wonderwall/i});
    await userEvent.click(screen.getByRole('button', {name: /add song/i}));
    await userEvent.type(screen.getByLabelText(/title/i), 'Creep');
    await userEvent.click(screen.getByRole('button', {name: /^add$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands/3/songs') &&
            init?.method === 'POST' &&
            String(init.body).includes('Creep'),
        ),
      ).toBe(true);
    });
  });
});
