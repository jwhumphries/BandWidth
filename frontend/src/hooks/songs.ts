import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {
  PracticeStats,
  Resource,
  SongDetail,
  SongListItem,
  SongStatus,
} from '../lib/types';

export function useSongs() {
  return useQuery<SongListItem[], ApiError>({
    queryKey: ['songs'],
    queryFn: () => api.get<SongListItem[]>('/api/songs'),
  });
}

export function useSong(id: number) {
  return useQuery<SongDetail, ApiError>({
    queryKey: ['songs', id],
    queryFn: () => api.get<SongDetail>(`/api/songs/${id}`),
  });
}

export function useCreateSong() {
  const queryClient = useQueryClient();
  return useMutation<SongDetail, ApiError, {title: string; artist: string}>({
    mutationFn: data => api.post<SongDetail>('/api/songs', data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['songs']}),
  });
}

export function useUpdateSong(id: number) {
  const queryClient = useQueryClient();
  return useMutation<
    SongDetail,
    ApiError,
    {title?: string; artist?: string; status?: SongStatus; notes?: string}
  >({
    mutationFn: data => api.patch<SongDetail>(`/api/songs/${id}`, data),
    onSuccess: detail => {
      queryClient.setQueryData(['songs', id], detail);
      void queryClient.invalidateQueries({queryKey: ['songs'], exact: true});
    },
  });
}

export function useDeleteSong() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/songs/${id}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: ['songs']});
      void queryClient.invalidateQueries({queryKey: ['folders']});
    },
  });
}

// applyStats patches a song's practice stats into both caches.
function applyStats(
  queryClient: ReturnType<typeof useQueryClient>,
  id: number,
  stats: PracticeStats,
) {
  queryClient.setQueryData<SongListItem[] | undefined>(['songs'], list =>
    list?.map(s => (s.id === id ? {...s, ...stats} : s)),
  );
  queryClient.setQueryData<SongDetail | undefined>(['songs', id], d =>
    d ? {...d, ...stats} : d,
  );
}

export function useLogPractice() {
  const queryClient = useQueryClient();
  return useMutation<PracticeStats, ApiError, {id: number; date: string}>({
    mutationFn: ({id, date}) =>
      api.put<PracticeStats>(`/api/songs/${id}/practice`, {date}),
    onSuccess: (stats, {id}) => applyStats(queryClient, id, stats),
  });
}

export function useUndoPractice() {
  const queryClient = useQueryClient();
  return useMutation<PracticeStats, ApiError, {id: number; date: string}>({
    mutationFn: ({id, date}) =>
      api.delete<PracticeStats>(`/api/songs/${id}/practice/${date}`),
    onSuccess: (stats, {id}) => applyStats(queryClient, id, stats),
  });
}

export function useCreateResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<Resource, ApiError, {url: string; label: string}>({
    mutationFn: data =>
      api.post<Resource>(`/api/songs/${songId}/resources`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}

export function useUpdateResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<
    Resource,
    ApiError,
    {id: number; url?: string; label?: string}
  >({
    mutationFn: ({id, ...data}) =>
      api.patch<Resource>(`/api/songs/${songId}/resources/${id}`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}

export function useDeleteResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/songs/${songId}/resources/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}
