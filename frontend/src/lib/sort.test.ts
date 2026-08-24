import {describe, expect, it} from 'vitest';
import {sortSongsByTitle} from './sort';

function song(id: number, title: string, artist = '') {
  return {id, title, artist};
}

describe('sortSongsByTitle', () => {
  it('sorts by title ignoring case, matching the server COLLATE NOCASE order', () => {
    const sorted = sortSongsByTitle([
      song(1, 'banjo'),
      song(2, 'Anvil'),
      song(3, 'Cymbal'),
    ]);
    expect(sorted.map(s => s.title)).toEqual(['Anvil', 'banjo', 'Cymbal']);
  });

  it('breaks title ties on artist, then id', () => {
    const sorted = sortSongsByTitle([
      song(7, 'Wish You Were Here', 'Pink Floyd'),
      song(3, 'Wish You Were Here', 'Pink Floyd'),
      song(5, 'Wish You Were Here', 'Incubus'),
    ]);
    expect(sorted.map(s => s.id)).toEqual([5, 3, 7]);
  });

  it('does not mutate the input array', () => {
    const songs = [song(1, 'Zebra'), song(2, 'Apple')];
    sortSongsByTitle(songs);
    expect(songs.map(s => s.title)).toEqual(['Zebra', 'Apple']);
  });
});
