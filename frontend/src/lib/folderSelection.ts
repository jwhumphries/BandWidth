import {useCallback, useEffect, useState} from 'react';
import type {Folder} from './types';

const PREFIX = 'bandwidth-folder:';

// Session storage throws in some privacy modes; a lost selection is not worth
// breaking the page over, so every access degrades to "no stored folder".
function read(scope: string): number | null {
  try {
    const stored = sessionStorage.getItem(PREFIX + scope);
    if (stored === null) return null;
    const id = Number(stored);
    return Number.isInteger(id) ? id : null;
  } catch {
    return null;
  }
}

function write(scope: string, id: number | null) {
  try {
    if (id === null) {
      sessionStorage.removeItem(PREFIX + scope);
    } else {
      sessionStorage.setItem(PREFIX + scope, String(id));
    }
  } catch {
    // Selection stays in memory for this page view only.
  }
}

/**
 * Folder selection for one library, remembered for the browser tab.
 *
 * Leaving a folder to open a song and coming back should land on the folder
 * you left, so the selection outlives the page component. `scope` keys the
 * storage per library (`band:7`, `personal`), and a stored folder missing
 * from `folders` — deleted in another tab, or in another session — falls
 * back to all songs. Pass `undefined` for `folders` while they load so a
 * valid selection is not discarded before the list arrives.
 */
export function useFolderSelection(
  scope: string,
  folders: Folder[] | undefined,
): [number | null, (id: number | null) => void] {
  const [selectedId, setSelectedId] = useState<number | null>(() =>
    read(scope),
  );

  // A new scope is a different library with its own remembered folder.
  useEffect(() => {
    setSelectedId(read(scope));
  }, [scope]);

  useEffect(() => {
    if (selectedId === null || !folders) return;
    if (!folders.some(f => f.id === selectedId)) {
      setSelectedId(null);
      write(scope, null);
    }
  }, [folders, scope, selectedId]);

  const select = useCallback(
    (id: number | null) => {
      setSelectedId(id);
      write(scope, id);
    },
    [scope],
  );

  return [selectedId, select];
}
