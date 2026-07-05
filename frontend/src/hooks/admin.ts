import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {AccessPolicy, AdminBand, AdminUser} from '../lib/types';

export function useAdminUsers() {
  return useQuery<AdminUser[], ApiError>({
    queryKey: ['admin', 'users'],
    queryFn: () => api.get<AdminUser[]>('/api/admin/users'),
  });
}

export function useDeleteAdminUser() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/users/${id}`),
    onSuccess: () => {
      // Deleting a user cascades to any band they created, so the Bands
      // tab can go stale too.
      void queryClient.invalidateQueries({queryKey: ['admin', 'users']});
      void queryClient.invalidateQueries({queryKey: ['admin', 'bands']});
    },
  });
}

export function useAdminBands() {
  return useQuery<AdminBand[], ApiError>({
    queryKey: ['admin', 'bands'],
    queryFn: () => api.get<AdminBand[]>('/api/admin/bands'),
  });
}

export function useDeleteAdminBand() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/bands/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['admin', 'bands']}),
  });
}

export function useAccessPolicy() {
  return useQuery<AccessPolicy, ApiError>({
    queryKey: ['admin', 'access-policy'],
    queryFn: () => api.get<AccessPolicy>('/api/admin/access-policy'),
  });
}

export function useSetAccessPolicy() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {enabled: boolean}>({
    mutationFn: data => api.put<void>('/api/admin/access-policy', data),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}

export function useAddAllowedEmail() {
  const queryClient = useQueryClient();
  return useMutation<{id: number; email: string}, ApiError, {email: string}>({
    mutationFn: data =>
      api.post<{id: number; email: string}>(
        '/api/admin/access-policy/emails',
        data,
      ),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}

export function useRemoveAllowedEmail() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/access-policy/emails/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}
