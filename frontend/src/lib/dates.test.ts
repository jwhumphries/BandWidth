import {describe, expect, it} from 'vitest';
import {localToday} from './dates';

describe('localToday', () => {
  it('returns a YYYY-MM-DD string for the local date', () => {
    const today = localToday();
    expect(today).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    const d = new Date();
    expect(today.startsWith(String(d.getFullYear()))).toBe(true);
  });
});
