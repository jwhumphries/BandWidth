export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body: unknown = null,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers: HeadersInit =
    init.body === undefined
      ? {...init.headers}
      : {'Content-Type': 'application/json', ...init.headers};
  const res = await fetch(path, {...init, headers});
  if (!res.ok) {
    let message = res.statusText;
    let body: unknown = null;
    try {
      body = await res.json();
      const m = (body as {message?: unknown}).message;
      if (typeof m === 'string') message = m;
    } catch {
      // non-JSON error body; keep statusText
    }
    throw new ApiError(res.status, message, body);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: data === undefined ? undefined : JSON.stringify(data),
    }),
  patch: <T>(path: string, data: unknown) =>
    request<T>(path, {method: 'PATCH', body: JSON.stringify(data)}),
  put: <T>(path: string, data: unknown) =>
    request<T>(path, {method: 'PUT', body: JSON.stringify(data)}),
};
