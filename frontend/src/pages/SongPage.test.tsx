import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import SongPage from './SongPage';

const detail = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  notes: 'capo 2',
  resources: [{id: 5, url: 'https://example.com/tab', label: 'tab'}],
  lastPracticedAt: '2026-06-10',
  practiceCount: 3,
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

function renderSongPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/songs/:id" element={<SongPage />} />
      <Route path="/" element={<p>home</p>} />
    </Routes>,
    {route: '/songs/1'},
  );
}

describe('SongPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          if (init?.method === 'PATCH') {
            return Promise.resolve(
              jsonResponse(200, {...detail, status: 'learned'}),
            );
          }
          if (init?.method === 'DELETE') {
            return Promise.resolve(new Response(null, {status: 204}));
          }
          if (url.includes('/api/folders')) {
            return Promise.resolve(jsonResponse(200, []));
          }
          return Promise.resolve(jsonResponse(200, detail));
        }),
    );
  });

  it('renders identity, notes, resources, and practice stats', async () => {
    renderSongPage();
    expect(await screen.findByDisplayValue('Wonderwall')).toBeInTheDocument();
    expect(screen.getByDisplayValue('capo 2')).toBeInTheDocument();
    expect(screen.getByText(/example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/3 days practiced/i)).toBeInTheDocument();
    expect(screen.getByText(/2026-06-10/)).toBeInTheDocument();
  });

  it('changes status via the select', async () => {
    renderSongPage();
    await screen.findByDisplayValue('Wonderwall');
    await userEvent.selectOptions(screen.getByLabelText(/status/i), 'learned');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([, init]) =>
            init?.method === 'PATCH' && String(init.body).includes('learned'),
        ),
      ).toBe(true);
    });
  });

  it('deletes with confirmation and navigates home', async () => {
    renderSongPage();
    await screen.findByDisplayValue('Wonderwall');
    await userEvent.click(screen.getByRole('button', {name: /delete song/i}));
    await userEvent.click(screen.getByRole('button', {name: /^delete$/i}));
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
  });
});
