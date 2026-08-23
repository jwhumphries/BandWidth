import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import BandFolderSidebar from './BandFolderSidebar';

const folders = [{id: 1, name: 'Set 1', position: 1, songIds: []}];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandFolderSidebar', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((_input, init?: RequestInit) => {
        if (init?.method === 'POST')
          return Promise.resolve(jsonResponse(201, {id: 2}));
        return Promise.resolve(jsonResponse(200, folders));
      }),
    );
  });

  it('lets editors create a folder', async () => {
    renderWithProviders(
      <BandFolderSidebar
        bandId={3}
        canEdit
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    await screen.findByText('Set 1');
    await userEvent.type(screen.getByPlaceholderText(/new folder/i), 'Encore');
    await userEvent.click(screen.getByRole('button', {name: /^create$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands/3/folders') &&
            init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });

  it('hides editing controls for viewers', async () => {
    renderWithProviders(
      <BandFolderSidebar
        bandId={3}
        canEdit={false}
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    await screen.findByText('Set 1');
    expect(
      screen.queryByPlaceholderText(/new folder/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /delete set 1/i}),
    ).not.toBeInTheDocument();
  });
});
