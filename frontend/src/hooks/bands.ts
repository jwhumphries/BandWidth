import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {
  BandDetail,
  BandInviteInfo,
  BandRole,
  BandSummary,
  CreatedInvite,
} from '../lib/types';

export function useBands() {
  return useQuery<BandSummary[], ApiError>({
    queryKey: ['bands'],
    queryFn: () => api.get<BandSummary[]>('/api/bands'),
  });
}

export function useBand(id: number) {
  return useQuery<BandDetail, ApiError>({
    queryKey: ['bands', id],
    queryFn: () => api.get<BandDetail>(`/api/bands/${id}`),
  });
}

export function useCreateBand() {
  const queryClient = useQueryClient();
  return useMutation<{id: number}, ApiError, {name: string}>({
    mutationFn: data => api.post<{id: number}>('/api/bands', data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useRenameBand(id: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {name: string}>({
    mutationFn: data => api.patch<void>(`/api/bands/${id}`, data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useDeleteBand() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/bands/${id}`),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useSetMemberRole(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {userId: number; role: BandRole}>({
    mutationFn: ({userId, role}) =>
      api.patch<void>(`/api/bands/${bandId}/members/${userId}`, {role}),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: ['bands', bandId]});
      void queryClient.invalidateQueries({queryKey: ['bands'], exact: true});
    },
  });
}

export function useRemoveMember(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: userId => api.delete(`/api/bands/${bandId}/members/${userId}`),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useBandInvites(bandId: number, enabled: boolean) {
  return useQuery<BandInviteInfo[], ApiError>({
    queryKey: ['bands', bandId, 'invites'],
    queryFn: () => api.get<BandInviteInfo[]>(`/api/bands/${bandId}/invites`),
    enabled,
  });
}

export function useCreateInvite(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<
    CreatedInvite,
    ApiError,
    {username?: string; role?: BandRole; link?: boolean}
  >({
    mutationFn: data =>
      api.post<CreatedInvite>(`/api/bands/${bandId}/invites`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'invites'],
      }),
  });
}

export function useRevokeInvite(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: inviteId =>
      api.delete(`/api/bands/${bandId}/invites/${inviteId}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['bands', bandId, 'invites'],
      }),
  });
}
