import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {
  BandSongDetail,
  Resource,
  SongListItem,
  SongStatus,
} from '../lib/types';

interface RehearsalStats {
  lastRehearsedAt: string;
  rehearsalCount: number;
}

export function useBandSongs(bandId: number, enabled = true) {
  return useQuery<SongListItem[], ApiError>({
    queryKey: ['bands', bandId, 'songs'],
    queryFn: () => api.get<SongListItem[]>(`/api/bands/${bandId}/songs`),
    enabled,
  });
}

export function useBandSong(bandId: number, songId: number) {
  return useQuery<BandSongDetail, ApiError>({
    queryKey: ['bands', bandId, 'songs', songId],
    queryFn: () =>
      api.get<BandSongDetail>(`/api/bands/${bandId}/songs/${songId}`),
  });
}

// invalidateBandSong refreshes the band song list, the band song detail, and
// the member library (band songs surface there too).
function invalidateBandSong(
  queryClient: ReturnType<typeof useQueryClient>,
  bandId: number,
  songId?: number,
) {
  void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'songs']});
  if (songId !== undefined) {
    void queryClient.invalidateQueries({
      queryKey: ['bands', bandId, 'songs', songId],
    });
  }
  void queryClient.invalidateQueries({queryKey: ['songs']});
}

export function useCreateBandSong(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<BandSongDetail, ApiError, {title: string; artist: string}>(
    {
      mutationFn: data =>
        api.post<BandSongDetail>(`/api/bands/${bandId}/songs`, data),
      onSuccess: () => invalidateBandSong(queryClient, bandId),
    },
  );
}

export function useUpdateBandSong(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<
    BandSongDetail,
    ApiError,
    {title?: string; artist?: string; status?: SongStatus; notes?: string}
  >({
    mutationFn: data =>
      api.patch<BandSongDetail>(`/api/bands/${bandId}/songs/${songId}`, data),
    onSuccess: detail => {
      queryClient.setQueryData(['bands', bandId, 'songs', songId], detail);
      invalidateBandSong(queryClient, bandId);
    },
  });
}

export function useDeleteBandSong(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: songId => api.delete(`/api/bands/${bandId}/songs/${songId}`),
    onSuccess: () => invalidateBandSong(queryClient, bandId),
  });
}

export function useLogBandRehearsal(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {date: string}>({
    mutationFn: data =>
      api.put<RehearsalStats>(
        `/api/bands/${bandId}/songs/${songId}/rehearsal`,
        data,
      ),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}

// List-scoped variants: the band view logs/undoes a rehearsal for any row
// without opening the song. Payload carries the songId.
export function useLogBandRehearsalInList(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {songId: number; date: string}>({
    mutationFn: ({songId, date}) =>
      api.put<RehearsalStats>(
        `/api/bands/${bandId}/songs/${songId}/rehearsal`,
        {date},
      ),
    onSuccess: (_data, {songId}) =>
      invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useUndoBandRehearsalInList(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {songId: number; date: string}>({
    mutationFn: ({songId, date}) =>
      api.delete<RehearsalStats>(
        `/api/bands/${bandId}/songs/${songId}/rehearsal/${date}`,
      ),
    onSuccess: (_data, {songId}) =>
      invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useCreateBandResource(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<Resource, ApiError, {url: string; label: string}>({
    mutationFn: data =>
      api.post<Resource>(
        `/api/bands/${bandId}/songs/${songId}/resources`,
        data,
      ),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useDeleteBandResource(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: resourceId =>
      api.delete(
        `/api/bands/${bandId}/songs/${songId}/resources/${resourceId}`,
      ),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}
