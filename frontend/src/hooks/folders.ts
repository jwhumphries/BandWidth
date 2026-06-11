import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {Folder} from '../lib/types';

export function useFolders() {
  return useQuery<Folder[], ApiError>({
    queryKey: ['folders'],
    queryFn: () => api.get<Folder[]>('/api/folders'),
  });
}

export function useCreateFolder() {
  const queryClient = useQueryClient();
  return useMutation<Folder, ApiError, {name: string}>({
    mutationFn: data => api.post<Folder>('/api/folders', data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useRenameFolder() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; name: string}>({
    mutationFn: ({id, name}) => api.patch<void>(`/api/folders/${id}`, {name}),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useDeleteFolder() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/folders/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useReorderFolders() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number[]>({
    mutationFn: folderIds => api.put<void>('/api/folders/order', {folderIds}),
    onMutate: folderIds => {
      // Optimistic: dnd already shows the new order; keep the cache in step.
      queryClient.setQueryData<Folder[] | undefined>(['folders'], folders => {
        if (!folders) return folders;
        const byID = new Map(folders.map(f => [f.id, f]));
        return folderIds
          .map(id => byID.get(id))
          .filter((f): f is Folder => f !== undefined);
      });
    },
    onError: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useSetFolderEntries() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; songIds: number[]}>({
    mutationFn: ({id, songIds}) =>
      api.put<void>(`/api/folders/${id}/entries`, {songIds}),
    onMutate: ({id, songIds}) => {
      queryClient.setQueryData<Folder[] | undefined>(['folders'], folders =>
        folders?.map(f => (f.id === id ? {...f, songIds} : f)),
      );
    },
    onError: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
    onSettled: () =>
      void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}
