import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import FolderSidebar from './FolderSidebar';

const folders = [{id: 1, name: 'Setlist', position: 1, songIds: []}];

describe('FolderSidebar', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          if (init?.method === 'POST') {
            return Promise.resolve(
              new Response(
                JSON.stringify({
                  id: 9,
                  name: 'New folder',
                  position: 2,
                  songIds: [],
                }),
                {status: 201},
              ),
            );
          }
          return Promise.resolve(
            new Response(JSON.stringify(folders), {status: 200}),
          );
        }),
    );
  });

  it('lists folders, selects one, and creates new ones', async () => {
    const onSelect = vi.fn();
    renderWithProviders(
      <FolderSidebar selectedId={null} onSelect={onSelect} />,
    );

    await userEvent.click(await screen.findByText('Setlist'));
    expect(onSelect).toHaveBeenCalledWith(1);

    await userEvent.type(screen.getByPlaceholderText(/new folder/i), 'Gigs');
    await userEvent.click(screen.getByRole('button', {name: /create/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(calls.some(([, init]) => init?.method === 'POST')).toBe(true);
    });
  });
});
