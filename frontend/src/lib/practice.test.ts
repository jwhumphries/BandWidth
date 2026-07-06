import {beforeEach, describe, expect, it} from 'vitest';
import type {Folder, SongListItem} from './types';
import {
  effectiveMode,
  isBandScoped,
  loadPracticeState,
  parseSource,
  resolveCandidates,
  savePracticeState,
  suggest,
} from './practice';

function song(id: number, lastPracticedAt: string, bandId?: number): SongListItem {
  return {
    id,
    title: `Song ${id}`,
    artist: 'A',
    status: 'learning',
    lastPracticedAt,
    practiceCount: 0,
    ...(bandId ? {bandId, bandName: 'Band'} : {}),
  };
}

const folder = (id: number, songIds: number[]): Folder => ({
  id,
  name: `F${id}`,
  position: id,
  songIds,
});

describe('parseSource', () => {
  it('decodes every source shape and falls back to all', () => {
    expect(parseSource('all')).toEqual({kind: 'all'});
    expect(parseSource('folder:7')).toEqual({kind: 'folder', folderId: 7});
    expect(parseSource('band:3')).toEqual({kind: 'band', bandId: 3});
    expect(parseSource('bandfolder:3:9')).toEqual({
      kind: 'bandfolder',
      bandId: 3,
      folderId: 9,
    });
    expect(parseSource('garbage')).toEqual({kind: 'all'});
  });
});

describe('isBandScoped / effectiveMode', () => {
  it('treats band and bandfolder as band-scoped only', () => {
    expect(isBandScoped('all')).toBe(false);
    expect(isBandScoped('folder:1')).toBe(false);
    expect(isBandScoped('band:2')).toBe(true);
    expect(isBandScoped('bandfolder:2:3')).toBe(true);
  });

  it('forces personal mode for non-band sources', () => {
    expect(effectiveMode('all', 'band')).toBe('personal');
    expect(effectiveMode('band:2', 'band')).toBe('band');
    expect(effectiveMode('band:2', 'personal')).toBe('personal');
  });
});

describe('suggest', () => {
  const rng = () => 0.5; // deterministic tie-break

  it('ranks never-practiced first, then oldest date, capped at count', () => {
    const candidates = [
      song(1, '2026-05-01'),
      song(2, ''),
      song(3, '2026-01-01'),
    ];
    expect(suggest(candidates, 2, rng)).toEqual([2, 3]);
  });

  it('returns all when fewer than count', () => {
    expect(suggest([song(1, ''), song(2, '2026-01-01')], 5, rng)).toEqual([
      1, 2,
    ]);
  });

  it('randomly picks among a tie group via the injected rng', () => {
    const tied = [song(1, ''), song(2, ''), song(3, '')];
    // rng returns ascending keys -> stable order 1,2,3; take first 2.
    let n = 0;
    const asc = () => (n++ % 3) / 3;
    expect(suggest(tied, 2, asc)).toEqual([1, 2]);
    // rng returns descending keys -> order reverses; take first 2.
    let m = 0;
    const desc = () => (2 - (m++ % 3)) / 3;
    expect(suggest(tied, 2, desc)).toEqual([3, 2]);
  });
});

describe('resolveCandidates', () => {
  const data = {
    songs: [song(1, ''), song(2, '2026-01-01', 3), song(3, '2026-02-01', 3)],
    folders: [folder(10, [1, 3])],
    bandSongs: [song(2, '2026-06-01'), song(3, '2026-06-02')],
    bandFolders: [folder(20, [3])],
  };

  it('all -> whole personal library', () => {
    expect(resolveCandidates({kind: 'all'}, 'personal', data).map(s => s.id)).toEqual([
      1, 2, 3,
    ]);
  });

  it('folder -> personal songs in the folder', () => {
    expect(
      resolveCandidates({kind: 'folder', folderId: 10}, 'personal', data).map(
        s => s.id,
      ),
    ).toEqual([1, 3]);
  });

  it('band + personal -> personal rows for that band', () => {
    const out = resolveCandidates({kind: 'band', bandId: 3}, 'personal', data);
    expect(out.map(s => s.id)).toEqual([2, 3]);
    expect(out[0].lastPracticedAt).toBe('2026-01-01'); // personal date
  });

  it('band + band -> band song list', () => {
    const out = resolveCandidates({kind: 'band', bandId: 3}, 'band', data);
    expect(out[0].lastPracticedAt).toBe('2026-06-01'); // band rehearsal date
  });

  it('bandfolder honors mode for the ranking source', () => {
    expect(
      resolveCandidates({kind: 'bandfolder', bandId: 3, folderId: 20}, 'personal', data)
        .map(s => s.id),
    ).toEqual([3]);
    expect(
      resolveCandidates({kind: 'bandfolder', bandId: 3, folderId: 20}, 'band', data)[0]
        .lastPracticedAt,
    ).toBe('2026-06-02');
  });
});

describe('load/save practice state', () => {
  beforeEach(() => localStorage.clear());

  it('returns defaults when nothing is stored', () => {
    expect(loadPracticeState()).toEqual({
      source: 'all',
      mode: 'personal',
      count: 3,
      generated: null,
    });
  });

  it('round-trips a saved state', () => {
    const state = {
      source: 'band:3',
      mode: 'band' as const,
      count: 5,
      generated: {source: 'band:3', mode: 'band' as const, list: [2, 3]},
    };
    savePracticeState(state);
    expect(loadPracticeState()).toEqual(state);
  });

  it('falls back to defaults on malformed storage', () => {
    localStorage.setItem('bandwidth:practice', '{not json');
    expect(loadPracticeState()).toEqual({
      source: 'all',
      mode: 'personal',
      count: 3,
      generated: null,
    });
  });
});
