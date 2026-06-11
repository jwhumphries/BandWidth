import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import AccountSettings from './AccountSettings';

const me = {
  id: 1,
  username: 'alice',
  email: 'alice@example.com',
  totpEnabled: false,
};

describe('AccountSettings', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          if (init?.method === 'PATCH') {
            return Promise.resolve(
              new Response(JSON.stringify({...me, username: 'alice2'}), {
                status: 200,
              }),
            );
          }
          return Promise.resolve(
            new Response(JSON.stringify(me), {status: 200}),
          );
        }),
    );
  });

  it('prefills the form from the current user and saves changes', async () => {
    renderWithProviders(<AccountSettings />);

    const username = await screen.findByLabelText(/username/i);
    await waitFor(() => expect(username).toHaveValue('alice'));
    expect(screen.getByLabelText(/email/i)).toHaveValue('alice@example.com');

    await userEvent.clear(username);
    await userEvent.type(username, 'alice2');
    await userEvent.click(screen.getByRole('button', {name: /save/i}));

    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent(/saved/i),
    );
  });
});
