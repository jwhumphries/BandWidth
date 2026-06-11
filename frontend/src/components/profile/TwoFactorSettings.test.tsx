import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import TwoFactorSettings from './TwoFactorSettings';

const me = {
  id: 1,
  username: 'alice',
  email: 'alice@example.com',
  totpEnabled: false,
};

describe('TwoFactorSettings', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/2fa/setup')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                secret: 'SECRET123',
                otpauthUrl: 'otpauth://totp/BandWidth:alice?secret=SECRET123',
              }),
              {status: 200},
            ),
          );
        }
        if (url.includes('/2fa/verify')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({backupCodes: ['AAAA-BBBB', 'CCCC-DDDD']}),
              {
                status: 200,
              },
            ),
          );
        }
        return Promise.resolve(new Response(JSON.stringify(me), {status: 200}));
      }),
    );
  });

  it('walks through enrollment and shows backup codes once', async () => {
    renderWithProviders(<TwoFactorSettings />);

    await userEvent.click(
      await screen.findByRole('button', {name: /enable 2fa/i}),
    );

    // Manual-entry secret appears (QR may render async).
    await waitFor(() =>
      expect(screen.getAllByText(/SECRET123/).length).toBeGreaterThan(0),
    );

    await userEvent.type(screen.getByLabelText(/^code$/i), '123456');
    await userEvent.click(screen.getByRole('button', {name: /confirm/i}));

    await waitFor(() =>
      expect(screen.getByText(/AAAA-BBBB/)).toBeInTheDocument(),
    );
    expect(screen.getByText(/will not be shown again/i)).toBeInTheDocument();

    // Dismissing the codes moves to the enabled (disable) state.
    await userEvent.click(screen.getByRole('button', {name: /i saved them/i}));
    await waitFor(() =>
      expect(
        screen.getByRole('button', {name: /disable 2fa/i}),
      ).toBeInTheDocument(),
    );
  });
});
