import {act, renderHook} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import {useFolderSelection} from './folderSelection';
import type {Folder} from './types';

function folder(id: number, name: string): Folder {
  return {id, name, position: id, songIds: []};
}

const folders = [folder(1, 'Set list'), folder(2, 'Learning')];

afterEach(() => {
  sessionStorage.clear();
  vi.restoreAllMocks();
});

describe('useFolderSelection', () => {
  it('starts on all songs when nothing is stored', () => {
    const {result} = renderHook(() => useFolderSelection('band:1', folders));
    expect(result.current[0]).toBeNull();
  });

  it('restores the stored folder for its scope on mount', () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '2');
    const {result} = renderHook(() => useFolderSelection('band:1', folders));
    expect(result.current[0]).toBe(2);
  });

  it('keeps each scope selection separate', () => {
    const {result} = renderHook(() => useFolderSelection('band:1', folders));
    act(() => result.current[1](1));

    const other = renderHook(() => useFolderSelection('band:9', folders));
    expect(other.result.current[0]).toBeNull();
    expect(sessionStorage.getItem('bandwidth-folder:band:1')).toBe('1');
  });

  it('re-reads storage when the scope changes', () => {
    sessionStorage.setItem('bandwidth-folder:band:2', '1');
    const {result, rerender} = renderHook(
      ({scope}) => useFolderSelection(scope, folders),
      {initialProps: {scope: 'band:1'}},
    );
    expect(result.current[0]).toBeNull();

    rerender({scope: 'band:2'});
    expect(result.current[0]).toBe(1);
  });

  it('falls back to all songs when the stored folder no longer exists', () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '99');
    const {result} = renderHook(() => useFolderSelection('band:1', folders));
    expect(result.current[0]).toBeNull();
    expect(sessionStorage.getItem('bandwidth-folder:band:1')).toBeNull();
  });

  it('keeps the stored folder while the folder list is still loading', () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '2');
    const {result} = renderHook(() => useFolderSelection('band:1', undefined));
    expect(result.current[0]).toBe(2);
  });

  it('clears the stored folder when all songs is selected', () => {
    sessionStorage.setItem('bandwidth-folder:band:1', '2');
    const {result} = renderHook(() => useFolderSelection('band:1', folders));

    act(() => result.current[1](null));

    expect(result.current[0]).toBeNull();
    expect(sessionStorage.getItem('bandwidth-folder:band:1')).toBeNull();
  });

  it('still selects folders when session storage is unavailable', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage disabled');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage disabled');
    });

    const {result} = renderHook(() => useFolderSelection('band:1', folders));
    expect(result.current[0]).toBeNull();

    act(() => result.current[1](1));
    expect(result.current[0]).toBe(1);
  });
});
