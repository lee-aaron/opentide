import { useStatus, useAuditLog, usePolicies, useRateLimitStatus } from '@/api/hooks'
import { Card, CardTitle, CardValue } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { formatTime, truncateHash } from '@/lib/utils'
import { Shield, Users, Puzzle, Clock, AlertTriangle, Activity, RefreshCw } from 'lucide-react'

export function DashboardPage() {
  const { data: status, isLoading: statusLoading, dataUpdatedAt } = useStatus()
  const { data: audit } = useAuditLog({ limit: 20 })
  const { data: policies } = usePolicies()
  const { data: rateLimit } = useRateLimitStatus()

  const mismatches = audit?.unacknowledged_mismatches ?? 0
  const totalEvents = audit?.total ?? 0
  const activePolicies = policies?.length ?? 0

  return (
    <div className="space-y-6">
      {/* TOCTOU Incident Banner */}
      {mismatches > 0 && (
        <div className="flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3">
          <AlertTriangle className="h-5 w-5 shrink-0 text-red-400" />
          <div className="flex-1">
            <div className="text-sm font-medium text-red-400">
              {mismatches} unacknowledged TOCTOU mismatch{mismatches > 1 ? 'es' : ''}
            </div>
            <div className="text-xs text-red-400/70">
              Action hash changed between approval and execution. Investigate in Audit Log.
            </div>
          </div>
          <a href="/admin/audit" className="rounded-lg border border-red-500/30 px-3 py-1 text-xs font-medium text-red-400 hover:bg-red-500/10">
            Investigate
          </a>
        </div>
      )}

      {/* Compact Stats Row */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <Card className="p-4">
          <CardTitle>Version</CardTitle>
          <CardValue className="text-xl">{statusLoading ? '-' : status?.version}</CardValue>
        </Card>
        <Card className="p-4">
          <CardTitle><Clock className="inline h-3 w-3 mr-1" />Uptime</CardTitle>
          <CardValue className="text-xl">{statusLoading ? '-' : status?.uptime}</CardValue>
        </Card>
        <Card className="p-4">
          <CardTitle><Users className="inline h-3 w-3 mr-1" />Tenants</CardTitle>
          <CardValue className="text-xl">{statusLoading ? '-' : status?.tenant_count}</CardValue>
        </Card>
        <Card className="p-4">
          <CardTitle><Puzzle className="inline h-3 w-3 mr-1" />Skills</CardTitle>
          <CardValue className="text-xl">{statusLoading ? '-' : status?.skill_count}</CardValue>
        </Card>
        <Card className="p-4">
          <CardTitle><Activity className="inline h-3 w-3 mr-1" />Policies</CardTitle>
          <CardValue className="text-xl">{activePolicies}</CardValue>
        </Card>
        <Card className="p-4">
          <CardTitle><Shield className="inline h-3 w-3 mr-1" />Rate Buckets</CardTitle>
          <CardValue className="text-xl">{rateLimit?.active_buckets ?? 0}</CardValue>
        </Card>
      </div>

      {/* Live Event Feed Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Shield className="h-5 w-5 text-sky-400" />
          <h2 className="text-sm font-medium text-slate-200">Live Security Events</h2>
          <span className="text-xs text-slate-500">({totalEvents} total)</span>
        </div>
        <div className="flex items-center gap-2 text-xs text-slate-600">
          <RefreshCw className="h-3 w-3" />
          {dataUpdatedAt ? `Updated ${formatTime(new Date(dataUpdatedAt).toISOString())}` : 'Loading...'}
        </div>
      </div>

      {/* Main Content: Security Event Stream */}
      {!audit?.entries?.length ? (
        status && status.tenant_count === 0 && status.skill_count === 0 ? (
          /* First-run onboarding */
          <Card className="border-sky-500/20 bg-sky-500/5">
            <h2 className="mb-3 text-lg font-medium text-sky-400">Getting Started</h2>
            <div className="space-y-3 text-sm text-slate-400">
              <div className="flex items-start gap-3">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sky-500/20 text-xs font-bold text-sky-400">1</span>
                <div>
                  <div className="font-medium text-slate-300">Configure a messaging adapter</div>
                  <div>Set <code className="text-sky-400">DISCORD_TOKEN</code> or <code className="text-sky-400">SLACK_BOT_TOKEN</code></div>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sky-500/20 text-xs font-bold text-sky-400">2</span>
                <div>
                  <div className="font-medium text-slate-300">Add skills to the skills/ directory</div>
                  <div>Each skill needs a <code className="text-sky-400">skill.yaml</code> manifest</div>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sky-500/20 text-xs font-bold text-sky-400">3</span>
                <div>
                  <div className="font-medium text-slate-300">Create tenants for your teams</div>
                  <div>Use the Tenants page to manage access and plan limits</div>
                </div>
              </div>
            </div>
          </Card>
        ) : (
          <Card>
            <p className="text-sm text-slate-500">No security events yet. Deploy a skill to see activity here.</p>
          </Card>
        )
      ) : (
        <Card className="p-0 overflow-hidden">
          <div className="divide-y divide-slate-700/50">
            {audit.entries.map((entry, i) => {
              const isMismatch = entry.actual_hash && entry.actual_hash !== entry.decision.hash
              return (
                <div
                  key={i}
                  className={`flex items-center justify-between px-4 py-3 text-xs ${
                    isMismatch ? 'bg-red-500/10' : 'hover:bg-slate-800/50'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Badge variant={entry.decision.allowed ? 'success' : 'danger'}>
                      {entry.decision.allowed ? 'ALLOW' : 'DENY'}
                    </Badge>
                    <span className="font-medium text-slate-200">{entry.action.skill_name}</span>
                    <span className="text-slate-500">{entry.action.action_type}</span>
                    <span className="hidden text-slate-600 sm:inline">→ {entry.action.target}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    {isMismatch && (
                      <Badge variant="danger">HASH MISMATCH</Badge>
                    )}
                    <span className="font-mono text-slate-600">{truncateHash(entry.decision.hash, 8)}</span>
                    <span className="text-slate-600">{formatTime(entry.timestamp)}</span>
                  </div>
                </div>
              )
            })}
          </div>
        </Card>
      )}

      {/* Active Policies Summary */}
      {(policies?.length ?? 0) > 0 && (
        <>
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-green-400" />
            <h2 className="text-sm font-medium text-slate-200">Active Approval Policies</h2>
          </div>
          <Card className="p-0 overflow-hidden">
            <div className="divide-y divide-slate-700/50">
              {policies!.slice(0, 5).map((p, i) => (
                <div key={i} className="flex items-center justify-between px-4 py-2.5 text-xs hover:bg-slate-800/50">
                  <div className="flex items-center gap-3">
                    <Badge variant={p.allowed ? 'success' : 'warning'}>
                      {p.allowed ? 'ALLOW' : 'PENDING'}
                    </Badge>
                    <span className="font-medium text-slate-300">{p.scope.skill_name}</span>
                    <span className="text-slate-500">{p.scope.action_type}</span>
                    <span className="text-slate-600">→ {p.scope.target}</span>
                  </div>
                  <span className="font-mono text-slate-600">{truncateHash(p.hash, 8)}</span>
                </div>
              ))}
            </div>
          </Card>
        </>
      )}
    </div>
  )
}
