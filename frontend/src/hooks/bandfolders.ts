import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {Folder} from '../lib/types';

export function useBandFolders(bandId: number, enabled = true) {
  return useQuery<Folder[], ApiError>({
    queryKey: ['bands', bandId, 'folders'],
    queryFn: () => api.get<Folder[]>(`/api/bands/${bandId}/folders`),
    enabled,
  });
}

export function useCreateBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<Folder, ApiError, {name: string}>({
    mutationFn: data => api.post<Folder>(`/api/bands/${bandId}/folders`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
  });
}

export function useRenameBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; name: string}>({
    mutationFn: ({id, name}) =>
      api.patch<void>(`/api/bands/${bandId}/folders/${id}`, {name}),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
  });
}

export function useDeleteBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/bands/${bandId}/folders/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
  });
}

export function useReorderBandFolders(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number[]>({
    mutationFn: folderIds =>
      api.put<void>(`/api/bands/${bandId}/folders/order`, {folderIds}),
    onMutate: folderIds => {
      queryClient.setQueryData<Folder[] | undefined>(
        ['bands', bandId, 'folders'],
        folders => {
          if (!folders) return folders;
          const byID = new Map(folders.map(f => [f.id, f]));
          return folderIds
            .map(id => byID.get(id))
            .filter((f): f is Folder => f !== undefined);
        },
      );
    },
    onError: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
  });
}

export function useSetBandFolderEntries(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; songIds: number[]}>({
    mutationFn: ({id, songIds}) =>
      api.put<void>(`/api/bands/${bandId}/folders/${id}/entries`, {songIds}),
    onMutate: ({id, songIds}) => {
      queryClient.setQueryData<Folder[] | undefined>(
        ['bands', bandId, 'folders'],
        folders => folders?.map(f => (f.id === id ? {...f, songIds} : f)),
      );
    },
    onError: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
    onSettled: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'folders'],
      }),
  });
}
