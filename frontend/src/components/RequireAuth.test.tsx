import {screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import RequireAuth from './RequireAuth';

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

  it('redirects to /login when unauthenticated', async () => {
    renderWithProviders(
      <Routes>
        <Route path="/login" element={<p>login page</p>} />
        <Route element={<RequireAuth />}>
          <Route path="/" element={<p>secret home</p>} />
        </Route>
      </Routes>,
    );
    await waitFor(() =>
      expect(screen.getByText(/login page/i)).toBeInTheDocument(),
    );
    expect(screen.queryByText(/secret home/i)).not.toBeInTheDocument();
  });
});
