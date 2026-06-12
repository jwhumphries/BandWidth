import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {MyInvite} from '../lib/types';

export function useMyInvites() {
  return useQuery<MyInvite[], ApiError>({
    queryKey: ['my-invites'],
    queryFn: () => api.get<MyInvite[]>('/api/invites'),
    staleTime: 60 * 1000,
  });
}

function useInviteResolution() {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({queryKey: ['my-invites']});
    void queryClient.invalidateQueries({queryKey: ['bands']});
    void queryClient.invalidateQueries({queryKey: ['songs']});
  };
}

export function useAcceptInvite() {
  const onResolved = useInviteResolution();
  return useMutation<{bandId: number}, ApiError, number>({
    mutationFn: id => api.post<{bandId: number}>(`/api/invites/${id}/accept`),
    onSuccess: onResolved,
  });
}

export function useDeclineInvite() {
  const onResolved = useInviteResolution();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.post<void>(`/api/invites/${id}/decline`),
    onSuccess: onResolved,
  });
}

export function useJoinByLink() {
  const onResolved = useInviteResolution();
  return useMutation<{bandId: number}, ApiError, string>({
    mutationFn: token =>
      api.post<{bandId: number}>(`/api/invites/link/${token}`),
    onSuccess: onResolved,
  });
}
