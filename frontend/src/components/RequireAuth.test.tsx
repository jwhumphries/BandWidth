import {screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes, useLocation} from 'react-router';
import {renderWithProviders} from '../test/utils';
import RequireAuth from './RequireAuth';

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="loc">{loc.pathname + loc.search}</span>;
}

describe('RequireAuth', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({message: 'authentication required'}), {
          status: 401,
        }),
      ),
    );
  });

  it('redirects to /login with a redirect param when unauthenticated', async () => {
    renderWithProviders(
      <Routes>
        <Route path="/login" element={<LocationProbe />} />
        <Route element={<RequireAuth />}>
          <Route path="/bands/3" element={<p>secret</p>} />
        </Route>
      </Routes>,
      {route: '/bands/3'},
    );
    await waitFor(() =>
      expect(screen.getByTestId('loc')).toHaveTextContent(
        '/login?redirect=%2Fbands%2F3',
      ),
    );
    expect(screen.queryByText('secret')).not.toBeInTheDocument();
  });

  it('offers a retry instead of redirecting on server errors', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({message: 'boom'}), {status: 500}),
        ),
    );
    renderWithProviders(
      <Routes>
        <Route path="/login" element={<p>login page</p>} />
        <Route element={<RequireAuth />}>
          <Route path="/" element={<p>secret home</p>} />
        </Route>
      </Routes>,
    );
    await waitFor(() =>
      expect(screen.getByRole('button', {name: /retry/i})).toBeInTheDocument(),
    );
    expect(screen.queryByText(/login page/i)).not.toBeInTheDocument();
  });
});
