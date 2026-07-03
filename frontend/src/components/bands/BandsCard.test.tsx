import {screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import BandsCard from './BandsCard';

const me = {
  id: 1,
  username: 'alice',
  email: 'alice@example.com',
  totpEnabled: false,
};
const bands = [
  {id: 1, name: 'Mine', creatorId: 1, role: 'admin', memberCount: 2},
  {id: 2, name: 'Theirs', creatorId: 9, role: 'editor', memberCount: 3},
];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandsCard', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/me')) {
          return Promise.resolve(jsonResponse(200, me));
        }
        return Promise.resolve(jsonResponse(200, bands));
      }),
    );
  });

  it('offers Leave only for bands the user did not create', async () => {
    renderWithProviders(<BandsCard />);
    const mineRow = (await screen.findByText('Mine')).closest('li')!;
    const theirsRow = screen.getByText('Theirs').closest('li')!;
    // The creator cannot leave their own band (they delete it instead).
    expect(within(mineRow).queryByText(/leave/i)).not.toBeInTheDocument();
    expect(within(theirsRow).getAllByText('Leave').length).toBeGreaterThan(0);
  });
});
