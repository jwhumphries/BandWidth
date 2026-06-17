// safeRedirect returns the given value only when it is a same-origin
// relative path (begins with a single "/", not "//" or "/\"), guarding
// against open redirects. Otherwise it returns "/".
export function safeRedirect(value: string | null): string {
  if (!value) return '/';
  if (value[0] !== '/') return '/';
  if (value[1] === '/' || value[1] === '\\') return '/';
  return value;
}
