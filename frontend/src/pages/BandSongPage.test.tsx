import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import BandSongPage from './BandSongPage';

const detail = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  notes: 'capo 2',
  resources: [{id: 5, url: 'https://example.com/tab', label: 'tab'}],
  lastRehearsedAt: '2026-06-10',
  rehearsalCount: 4,
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

// The band detail (for myRole) plus the band-song detail share one stub.
function stubFetch(
  myRole: string,
  folders: Array<{
    id: number;
    name: string;
    position: number;
    songIds: number[];
  }> = [],
) {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/folders/') && init?.method === 'PUT') {
          return Promise.resolve(new Response(null, {status: 204}));
        }
        if (url.includes('/folders')) {
          return Promise.resolve(jsonResponse(200, folders));
        }
        if (init?.method === 'PATCH') {
          return Promise.resolve(
            jsonResponse(200, {...detail, status: 'nailed'}),
          );
        }
        if (init?.method === 'PUT' || init?.method === 'POST') {
          return Promise.resolve(
            jsonResponse(200, {
              lastRehearsedAt: '2026-06-11',
              rehearsalCount: 5,
            }),
          );
        }
        if (url.includes('/songs/1')) {
          return Promise.resolve(jsonResponse(200, detail));
        }
        // band detail
        return Promise.resolve(
          jsonResponse(200, {
            id: 3,
            name: 'The Quietones',
            creatorId: 1,
            myRole,
            members: [],
          }),
        );
      }),
  );
}

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/bands/:id/songs/:songId" element={<BandSongPage />} />
      <Route path="/bands/:id" element={<p>band page</p>} />
    </Routes>,
    {route: '/bands/3/songs/1'},
  );
}

describe('BandSongPage', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('shows the band layer and lets editors change status', async () => {
    stubFetch('admin');
    renderPage();
    expect(await screen.findByDisplayValue('Wonderwall')).toBeInTheDocument();
    expect(screen.getByText(/4 rehearsals/i)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText(/status/i), 'nailed');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/bands/3/songs/1') &&
            init?.method === 'PATCH' &&
            String(init.body).includes('nailed'),
        ),
      ).toBe(true);
    });
  });

  it('renders read-only for viewers (no status control, no delete)', async () => {
    stubFetch('viewer');
    renderPage();
    await screen.findByText('Wonderwall');
    expect(screen.queryByLabelText(/status/i)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /delete song/i}),
    ).not.toBeInTheDocument();
  });

  it('shows the folders section and toggles band folder membership for editors', async () => {
    stubFetch('admin', [{id: 10, name: 'Setlist', position: 1, songIds: []}]);
    renderPage();
    await screen.findByDisplayValue('Wonderwall');
    const setlist = await screen.findByLabelText('Setlist');
    expect(setlist).not.toBeChecked();

    await userEvent.click(setlist);
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/bands/3/folders/10/entries') &&
            init?.method === 'PUT' &&
            String(init.body).includes('1'),
        ),
      ).toBe(true);
    });
  });

  it('logs a rehearsal for a backfilled past date', async () => {
    stubFetch('admin');
    renderPage();
    await screen.findByDisplayValue('Wonderwall');

    const logPastDay = screen.getByRole('button', {name: /log past day/i});
    expect(logPastDay).toBeDisabled();

    await userEvent.type(screen.getByLabelText(/backfill date/i), '2026-05-01');
    expect(logPastDay).toBeEnabled();
    await userEvent.click(logPastDay);

    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/bands/3/songs/1/rehearsal') &&
            init?.method === 'PUT' &&
            String(init.body).includes('2026-05-01'),
        ),
      ).toBe(true);
    });
  });
});
