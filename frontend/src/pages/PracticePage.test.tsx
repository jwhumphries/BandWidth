import {cleanup, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {localToday} from '../lib/dates';
import {renderWithProviders} from '../test/utils';
import PracticePage from './PracticePage';

const today = localToday();

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

const songs = [
  {
    id: 1,
    title: 'Alpha',
    artist: 'A',
    status: 'learning',
    lastPracticedAt: '',
    practiceCount: 0,
  },
  {
    id: 2,
    title: 'Bravo',
    artist: 'B',
    status: 'learned',
    lastPracticedAt: '2026-01-01',
    practiceCount: 3,
  },
  {
    id: 3,
    title: 'Charlie',
    artist: 'C',
    status: 'nailed',
    lastPracticedAt: today,
    practiceCount: 5,
  },
];

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/api/songs/') && init?.method === 'PUT') {
          return Promise.resolve(
            json(200, {lastPracticedAt: today, practiceCount: 1}),
          );
        }
        if (url.includes('/api/songs/') && init?.method === 'DELETE') {
          return Promise.resolve(
            json(200, {lastPracticedAt: '', practiceCount: 0}),
          );
        }
        if (url.endsWith('/api/songs'))
          return Promise.resolve(json(200, songs));
        return Promise.resolve(json(200, [])); // /api/folders, /api/bands
      }),
  );
}

function rowFor(title: string) {
  return screen.getByText(title).closest('li') as HTMLElement;
}

describe('PracticePage', () => {
  beforeEach(() => {
    localStorage.clear();
    stubFetch();
  });

  it('suggests least-recently-practiced songs and marks today as done', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));

    // Never-practiced Alpha first, then Bravo; Charlie (today) shows as done.
    expect(await screen.findByText('Alpha')).toBeInTheDocument();
    expect(
      within(rowFor('Alpha')).getByRole('button', {name: /practiced/i}),
    ).toBeInTheDocument();
    expect(
      within(rowFor('Charlie')).getByRole('button', {name: /undo/i}),
    ).toBeInTheDocument();
  });

  it('logs practice with today when a suggested song is tapped', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Alpha');

    await userEvent.click(
      within(rowFor('Alpha')).getByRole('button', {name: /practiced/i}),
    );
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/api/songs/1/practice') &&
            init?.method === 'PUT' &&
            String(init.body).includes(today),
        ),
      ).toBe(true);
    });
  });

  it('confirms before undoing an already-practiced song', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Charlie');

    await userEvent.click(
      within(rowFor('Charlie')).getByRole('button', {name: /undo/i}),
    );
    // Confirm modal opens; confirm it.
    await userEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', {name: /undo/i}),
    );
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes(`/api/songs/3/practice/${today}`) &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });

  it('restores the previous suggestion on remount without regenerating', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Alpha');

    cleanup();
    renderWithProviders(<PracticePage />);
    // No Suggest click this time — list comes back from localStorage.
    expect(await screen.findByText('Alpha')).toBeInTheDocument();
  });
});
