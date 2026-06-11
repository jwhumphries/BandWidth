import {beforeEach, describe, expect, it, vi} from 'vitest';
import {api, ApiError} from './api';

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  it('returns parsed JSON on success', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({id: 1}), {status: 200}),
    );
    await expect(api.get('/api/me')).resolves.toEqual({id: 1});
  });

  it('returns undefined for 204 responses', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(null, {status: 204}));
    await expect(api.post('/api/auth/logout')).resolves.toBeUndefined();
  });

  it('throws ApiError with server message and body', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({message: 'nope', totpRequired: true}), {
        status: 401,
      }),
    );
    const err = await api.get('/api/me').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    expect((err as ApiError).message).toBe('nope');
    expect(
      ((err as ApiError).body as {totpRequired: boolean}).totpRequired,
    ).toBe(true);
  });

  it('sends JSON bodies with content-type', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({}), {status: 200}),
    );
    await api.post('/api/auth/login', {login: 'a'});
    const [, init] = vi.mocked(fetch).mock.calls[0]!;
    expect(init?.method).toBe('POST');
    expect(init?.body).toBe('{"login":"a"}');
  });
});
