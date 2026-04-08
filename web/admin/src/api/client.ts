import { useAuthStore } from '@/store/auth'

const BASE = '/admin/api'

class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (res.status === 401) {
    useAuthStore.getState().setAuth(false)
    throw new ApiError(401, 'Authentication required')
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }))
    throw new ApiError(res.status, body.message || body.error || res.statusText)
  }

  return res.json()
}

export const api = {
  // Auth
  login: (secret: string) =>
    request<{ status: string }>('/login', {
      method: 'POST',
      body: JSON.stringify({ secret }),
    }),
  logout: () =>
    request<{ status: string }>('/logout', { method: 'POST' }),
  me: () =>
    request<{ authenticated: boolean; demo?: boolean }>('/me'),
  authConfig: () =>
    request<{ google_enabled: boolean; secret_enabled: boolean; demo_mode: boolean }>('/auth/config'),

  // Dashboard
  status: () => request<import('./types').StatusResponse>('/status'),

  // Tenants
  listTenants: () => request<import('./types').Tenant[]>('/tenants'),
  getTenant: (id: string) => request<import('./types').Tenant>(`/tenants/${id}`),
  createTenant: (t: Partial<import('./types').Tenant>) =>
    request<import('./types').Tenant>('/tenants', {
      method: 'POST',
      body: JSON.stringify(t),
    }),
  updateTenant: (id: string, t: Partial<import('./types').Tenant>) =>
    request<import('./types').Tenant>(`/tenants/${id}`, {
      method: 'PUT',
      body: JSON.stringify(t),
    }),
  deleteTenant: (id: string) =>
    request<{ status: string }>(`/tenants/${id}`, { method: 'DELETE' }),

  // Skills
  listSkills: () => request<import('./types').SkillInfo[]>('/skills'),

  // Approvals
  listPolicies: () => request<import('./types').ApprovalPolicy[]>('/approvals/policies'),
  createPolicy: (policy: { skill_name: string; action_type: string; target: string; allowed: boolean; reason: string }) =>
    request<import('./types').ApprovalPolicy>('/approvals/policies', {
      method: 'POST',
      body: JSON.stringify(policy),
    }),
  deletePolicy: (key: string) =>
    request<{ status: string }>(`/approvals/policies/${encodeURIComponent(key)}`, { method: 'DELETE' }),
  auditLog: (params?: { offset?: number; limit?: number; skill?: string; mismatch?: boolean }) => {
    const q = new URLSearchParams()
    if (params?.offset) q.set('offset', String(params.offset))
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.skill) q.set('skill', params.skill)
    if (params?.mismatch) q.set('mismatch', 'true')
    return request<import('./types').AuditLogResponse>(`/approvals/audit?${q}`)
  },

  // Security
  rateLimitStatus: () => request<import('./types').RateLimitStats>('/security/ratelimit'),

  // Audit actions
  acknowledgeAudit: (index: number) =>
    request<{ status: string }>(`/approvals/audit/${index}/acknowledge`, { method: 'POST' }),

  // Config
  getConfig: () => request<import('./types').GatewayConfig>('/config'),
  getProviders: () => request<import('./types').ProviderInfo[]>('/config/providers'),

  // Provider routing
  listProviders: () => request<import('./types').ProviderInfo[]>('/providers'),
  listRoutes: () => request<import('./types').ProviderRoute[]>('/providers/routes'),
  createRoute: (route: import('./types').ProviderRoute) =>
    request<import('./types').ProviderRoute>('/providers/routes', {
      method: 'POST',
      body: JSON.stringify(route),
    }),
  deleteRoute: (index: number) =>
    request<{ status: string }>(`/providers/routes/${index}`, { method: 'DELETE' }),
  testRoute: (userID: string, channelID: string) =>
    request<import('./types').TestRouteResult>('/providers/test-route', {
      method: 'POST',
      body: JSON.stringify({ user_id: userID, channel_id: channelID }),
    }),

  // Secrets (API key management)
  listSecrets: () => request<import('./types').SecretMeta[]>('/secrets'),
  setSecret: (req: import('./types').SetSecretRequest) =>
    request<import('./types').SecretMeta>('/secrets', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  deleteSecret: (provider: string) =>
    request<{ status: string }>(`/secrets/${provider}`, { method: 'DELETE' }),
}
