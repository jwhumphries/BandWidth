import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import LoginPage from './LoginPage';

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        if (String(input).includes('/api/auth/features')) {
          return Promise.resolve(jsonResponse(200, {passwordReset: false}));
        }
        return Promise.resolve(jsonResponse(404, {message: 'not found'}));
      }),
    );
  });

  it('shows an error for bad credentials', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('/api/auth/login')) {
        return Promise.resolve(
          jsonResponse(401, {message: 'invalid credentials'}),
        );
      }
      return Promise.resolve(jsonResponse(200, {passwordReset: false}));
    });
    renderWithProviders(<LoginPage />);

    await userEvent.type(screen.getByLabelText(/username or email/i), 'alice');
    await userEvent.type(screen.getByLabelText(/^password$/i), 'wrong');
    await userEvent.click(screen.getByRole('button', {name: /log in/i}));

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(
        /invalid credentials/i,
      ),
    );
  });

  it('reveals the two-factor field when required', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('/api/auth/login')) {
        return Promise.resolve(
          jsonResponse(401, {
            message: 'two-factor code required',
            totpRequired: true,
          }),
        );
      }
      return Promise.resolve(jsonResponse(200, {passwordReset: false}));
    });
    renderWithProviders(<LoginPage />);

    expect(screen.queryByLabelText(/two-factor code/i)).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText(/username or email/i), 'alice');
    await userEvent.type(
      screen.getByLabelText(/^password$/i),
      'hunter2hunter2',
    );
    await userEvent.click(screen.getByRole('button', {name: /log in/i}));

    await waitFor(() =>
      expect(screen.getByLabelText(/two-factor code/i)).toBeInTheDocument(),
    );
    // The "code required" response is a flow signal, not an error banner.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('hides the forgot-password link when the feature is off', async () => {
    renderWithProviders(<LoginPage />);
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(screen.queryByText(/forgot password/i)).not.toBeInTheDocument();
  });
});
