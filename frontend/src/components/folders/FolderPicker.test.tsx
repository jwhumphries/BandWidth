import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import FolderPicker from './FolderPicker';

const folders = [
  {id: 1, name: 'Setlist', position: 1, songIds: [7]},
  {id: 2, name: 'Queue', position: 2, songIds: []},
];

describe('FolderPicker', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          if (init?.method === 'PUT') {
            return Promise.resolve(new Response(null, {status: 204}));
          }
          return Promise.resolve(
            new Response(JSON.stringify(folders), {status: 200}),
          );
        }),
    );
  });

  it('checks folders containing the song and toggles membership', async () => {
    renderWithProviders(<FolderPicker songId={7} />);
    const setlist = await screen.findByLabelText('Setlist');
    const queue = screen.getByLabelText('Queue');
    expect(setlist).toBeChecked();
    expect(queue).not.toBeChecked();

    await userEvent.click(queue);
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/api/folders/2/entries') &&
            init?.method === 'PUT' &&
            String(init.body).includes('7'),
        ),
      ).toBe(true);
    });
  });
});
