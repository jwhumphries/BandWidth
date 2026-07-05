import {screen, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import RequireAdmin from './RequireAdmin';

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('RequireAdmin', () => {
  it('renders the nested route when the user is an admin', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          id: 1,
          username: 'admin',
          email: 'a@b.c',
          totpEnabled: false,
          isAdmin: true,
        }),
      ),
    );
    renderWithProviders(
      <Routes>
        <Route element={<RequireAdmin />}>
          <Route path="/" element={<p>admin area</p>} />
        </Route>
      </Routes>,
    );
    expect(await screen.findByText('admin area')).toBeInTheDocument();
  });

  it('redirects home when the user is not an admin', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          id: 1,
          username: 'bob',
          email: 'b@b.c',
          totpEnabled: false,
          isAdmin: false,
        }),
      ),
    );
    renderWithProviders(
      <Routes>
        <Route path="/" element={<p>home</p>} />
        <Route element={<RequireAdmin />}>
          <Route path="/admin" element={<p>admin area</p>} />
        </Route>
      </Routes>,
      {route: '/admin'},
    );
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
    expect(screen.queryByText('admin area')).not.toBeInTheDocument();
  });
});
