import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type { Tenant, SetSecretRequest, SetAdapterSecretRequest } from './types'

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
    queryFn: api.listProviders,
    refetchInterval: 30_000,
  })
}

export function useRoutes() {
  return useQuery({
    queryKey: ['routes'],
    queryFn: api.listRoutes,
  })
}

export function useCreateRoute() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (route: import('./types').ProviderRoute) => api.createRoute(route),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['routes'] }),
  })
}

export function useDeleteRoute() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (index: number) => api.deleteRoute(index),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['routes'] }),
  })
}

export function useTestRoute() {
  return useMutation({
    mutationFn: ({ userID, channelID }: { userID: string; channelID: string }) =>
      api.testRoute(userID, channelID),
  })
}

export function useSecrets() {
  return useQuery({
    queryKey: ['secrets'],
    queryFn: api.listSecrets,
    refetchInterval: 30_000,
  })
}

export function useSetSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: SetSecretRequest) => api.setSecret(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['secrets'] })
      qc.invalidateQueries({ queryKey: ['providers'] })
    },
  })
}

export function useDeleteSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (provider: string) => api.deleteSecret(provider),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['secrets'] })
      qc.invalidateQueries({ queryKey: ['providers'] })
    },
  })
}

// Adapter secrets
export function useAdapterSecrets() {
  return useQuery({
    queryKey: ['adapterSecrets'],
    queryFn: api.listAdapterSecrets,
    refetchInterval: 30_000,
  })
}

export function useSetAdapterSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: SetAdapterSecretRequest) => api.setAdapterSecret(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['adapterSecrets'] }),
  })
}

export function useDeleteAdapterSecret() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (adapter: string) => api.deleteAdapterSecret(adapter),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['adapterSecrets'] }),
  })
}

// Models
export function useModels(provider: string) {
  return useQuery({
    queryKey: ['models', provider],
    queryFn: () => api.listModels(provider),
    enabled: !!provider,
  })
}

export function useSetModel() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ provider, model }: { provider: string; model: string }) =>
      api.setModel(provider, model),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['providers'] })
      qc.invalidateQueries({ queryKey: ['models'] })
    },
  })
}

// Skill toggle
export function useToggleSkill() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ toolName, enabled }: { toolName: string; enabled: boolean }) =>
      api.toggleSkill(toolName, enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['skills'] }),
  })
}
