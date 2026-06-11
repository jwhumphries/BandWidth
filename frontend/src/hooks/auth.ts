import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useNavigate} from 'react-router';
import {api, ApiError} from '../lib/api';
import type {
  AuthFeatures,
  TwoFactorSetupResponse,
  TwoFactorVerifyResponse,
  User,
} from '../lib/types';

export function useMe() {
  return useQuery<User, ApiError>({
    queryKey: ['me'],
    queryFn: () => api.get<User>('/api/me'),
    retry: false,
    staleTime: 5 * 60 * 1000,
  });
}

export function useAuthFeatures() {
  return useQuery<AuthFeatures, ApiError>({
    queryKey: ['auth-features'],
    queryFn: () => api.get<AuthFeatures>('/api/auth/features'),
    staleTime: Infinity,
  });
}

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation<
    User,
    ApiError,
    {login: string; password: string; totpCode?: string}
  >({
    mutationFn: data => api.post<User>('/api/auth/login', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useSignup() {
  const queryClient = useQueryClient();
  return useMutation<
    User,
    ApiError,
    {username: string; email: string; password: string}
  >({
    mutationFn: data => api.post<User>('/api/auth/signup', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => api.post<void>('/api/auth/logout'),
    onSettled: () => {
      queryClient.clear();
      void navigate('/login');
    },
  });
}

export function useUpdateMe() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {username?: string; email?: string}>({
    mutationFn: data => api.patch<User>('/api/me', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useChangePassword() {
  const queryClient = useQueryClient();
  return useMutation<
    User,
    ApiError,
    {currentPassword: string; newPassword: string}
  >({
    mutationFn: data => api.put<User>('/api/me/password', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useTwoFactorSetup() {
  return useMutation<TwoFactorSetupResponse, ApiError, void>({
    mutationFn: () => api.post<TwoFactorSetupResponse>('/api/auth/2fa/setup'),
  });
}

export function useTwoFactorVerify() {
  const queryClient = useQueryClient();
  return useMutation<TwoFactorVerifyResponse, ApiError, {code: string}>({
    mutationFn: data =>
      api.post<TwoFactorVerifyResponse>('/api/auth/2fa/verify', data),
    onSuccess: () => {
      queryClient.setQueryData<User | undefined>(['me'], u =>
        u ? {...u, totpEnabled: true} : u,
      );
    },
  });
}

export function useTwoFactorDisable() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {code: string}>({
    mutationFn: data => api.post<User>('/api/auth/2fa/disable', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useRequestPasswordReset() {
  return useMutation<void, ApiError, {email: string}>({
    mutationFn: data => api.post<void>('/api/auth/password-reset', data),
  });
}

export function useConfirmPasswordReset() {
  return useMutation<void, ApiError, {token: string; newPassword: string}>({
    mutationFn: data =>
      api.post<void>('/api/auth/password-reset/confirm', data),
  });
}
