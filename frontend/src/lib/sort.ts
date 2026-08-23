interface SortableSong {
  id: number;
  title: string;
  artist: string;
}

/**
 * Returns a new array of songs ordered by title, then artist, then id.
 *
 * Song lists are alphabetical everywhere — library, personal folders, and
 * band folders alike. Folder membership carries no order of its own, so the
 * client sorts folder contents to match the `title COLLATE NOCASE, id`
 * ordering the server already applies to full song lists.
 */
export function sortSongsByTitle<T extends SortableSong>(songs: T[]): T[] {
  const compare = (a: string, b: string) =>
    a.localeCompare(b, undefined, {sensitivity: 'base'});
  return [...songs].sort(
    (a, b) =>
      compare(a.title, b.title) || compare(a.artist, b.artist) || a.id - b.id,
  );
}
