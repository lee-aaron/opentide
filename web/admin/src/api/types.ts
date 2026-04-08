export interface StatusResponse {
  version: string
  uptime: string
  tenant_count: number
  skill_count: number
  server_time: string
}

export interface Tenant {
  id: string
  name: string
  platform: string
  platform_id: string
  plan: TenantPlan
  enabled: boolean
}

export interface TenantPlan {
  name: string
  max_messages_per_day: number
  max_skills: number
  allowed_skills: string[]
}

export interface SkillInfo {
  name: string
  version: string
  description: string
  tool_name: string
  loaded: boolean
  enabled: boolean
  security?: SkillSecurity
}

export interface SkillSecurity {
  egress: string[]
  filesystem: string
  max_memory: string
  max_cpu: string
  timeout: string
}

export interface ApprovalPolicy {
  allowed: boolean
  reason: string
  hash: string
  expires_at: string
  scope: {
    skill_name: string
    action_type: string
    target: string
  }
}

export interface AuditEntry {
  timestamp: string
  action: {
    skill_name: string
    skill_version: string
    action_type: string
    target: string
    payload: string
  }
  approved_action?: {
    skill_name: string
    skill_version: string
    action_type: string
    target: string
    payload: string
  }
  decision: ApprovalPolicy
  actual_hash?: string
  acknowledged?: boolean
}

export interface AuditLogResponse {
  entries: AuditEntry[] | null
  total: number
  unacknowledged_mismatches: number
}

export interface RateLimitStats {
  active_buckets: number
  config: {
    rate: number
    burst: number
    cleanup_age: number
  }
}

export interface AuthStatus {
  authenticated: boolean
  demo?: boolean
}

export interface GatewayConfig {
  gateway: {
    host: string
    port: number
    log_level: string
    demo_mode: boolean
    dev_mode: boolean
  }
  state: {
    driver: string
  }
  security: {
    max_message_size: number
    approval_ttl: number
    admin_port: number
    admin_secret: string
  }
}

export interface ProviderInfo {
  name: string
  model: string
  configured: boolean
  healthy: boolean
  is_default: boolean
}

export interface ProviderRoute {
  channel_id?: string
  tenant_id?: string
  provider: string
  model?: string
  priority: number
  security?: RoutePolicy
}

export interface RoutePolicy {
  max_tokens_per_request?: number
  audit_verbosity?: string
}

export interface TestRouteResult {
  resolved: boolean
  provider?: string
  model?: string
  user_id: string
  channel_id: string
  has_override?: boolean
  override_name?: string
}

export interface SecretMeta {
  provider: string
  last4: string
  created_at: string
  source: 'env' | 'store'
  configured: boolean
}

export interface SetSecretRequest {
  provider: string
  api_key: string
  model?: string
  base_url?: string
}

export interface SetAdapterSecretRequest {
  adapter: string
  token: string
}

export interface ModelInfo {
  id: string
  display_name?: string
  current: boolean
}
