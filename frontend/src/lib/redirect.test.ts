import {describe, expect, it} from 'vitest';
import {safeRedirect} from './redirect';

describe('safeRedirect', () => {
  it('allows same-origin relative paths', () => {
    expect(safeRedirect('/join/abc')).toBe('/join/abc');
    expect(safeRedirect('/bands/3')).toBe('/bands/3');
  });

  it('rejects protocol-relative and absolute URLs', () => {
    expect(safeRedirect('//evil.com')).toBe('/');
    expect(safeRedirect('https://evil.com')).toBe('/');
    expect(safeRedirect('/\\evil.com')).toBe('/');
  });

  it('defaults to / for null, empty, or non-paths', () => {
    expect(safeRedirect(null)).toBe('/');
    expect(safeRedirect('')).toBe('/');
    expect(safeRedirect('join/abc')).toBe('/');
  });
});
