interface SortableSong {
  id: number;
  title: string;
  artist: string;
}

/**
 * Returns a new array of songs ordered by title, then artist, then id.
 *
 * Song lists are alphabetical everywhere — library, personal folders, and
 * band folders alike. Folder membership carries no order of its own, so every
 * list is sorted here rather than relying on the server's `title COLLATE
 * NOCASE, id`; artist breaks a title tie so same-named covers group by who
 * plays them instead of by insertion order.
 */
export function sortSongsByTitle<T extends SortableSong>(songs: T[]): T[] {
  const compare = (a: string, b: string) =>
    a.localeCompare(b, undefined, {sensitivity: 'base'});
  return [...songs].sort(
    (a, b) =>
      compare(a.title, b.title) || compare(a.artist, b.artist) || a.id - b.id,
  );
}
