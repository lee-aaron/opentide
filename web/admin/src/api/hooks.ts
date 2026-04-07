import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type { Tenant } from './types'

export function useStatus() {
  return useQuery({
    queryKey: ['status'],
    queryFn: api.status,
    refetchInterval: 30_000,
  })
}

export function useTenants() {
  return useQuery({
    queryKey: ['tenants'],
    queryFn: api.listTenants,
  })
}

export function useTenant(id: string) {
  return useQuery({
    queryKey: ['tenants', id],
    queryFn: () => api.getTenant(id),
    enabled: !!id,
  })
}

export function useCreateTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (t: Partial<Tenant>) => api.createTenant(t),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tenants'] }),
  })
}

export function useDeleteTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteTenant(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tenants'] }),
  })
}

export function useSkills() {
  return useQuery({
    queryKey: ['skills'],
    queryFn: api.listSkills,
  })
}

export function usePolicies() {
  return useQuery({
    queryKey: ['policies'],
    queryFn: api.listPolicies,
  })
}

export function useCreatePolicy() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (policy: { skill_name: string; action_type: string; target: string; allowed: boolean; reason: string }) =>
      api.createPolicy(policy),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies'] }),
  })
}

export function useDeletePolicy() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (key: string) => api.deletePolicy(key),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies'] }),
  })
}

export function useAuditLog(params?: { offset?: number; limit?: number; skill?: string; mismatch?: boolean }) {
  return useQuery({
    queryKey: ['audit', params],
    queryFn: () => api.auditLog(params),
    refetchInterval: 30_000,
  })
}

export function useRateLimitStatus() {
  return useQuery({
    queryKey: ['ratelimit'],
    queryFn: api.rateLimitStatus,
  })
}

export function useAcknowledgeAudit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (index: number) => api.acknowledgeAudit(index),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['audit'] }),
  })
}

export function useConfig() {
  return useQuery({
    queryKey: ['config'],
    queryFn: api.getConfig,
  })
}

export function useProviders() {
  return useQuery({
    queryKey: ['providers'],
    queryFn: api.getProviders,
  })
}
