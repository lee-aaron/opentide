import { useRateLimitStatus, useSkills } from '@/api/hooks'
import { Card, CardTitle, CardValue } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Shield, Globe, Lock } from 'lucide-react'

export function SecurityPage() {
  const { data: rateLimit, isLoading: rlLoading } = useRateLimitStatus()
  const { data: skills, isLoading: skillsLoading } = useSkills()

  const skillsWithEgress = skills?.filter((s) => s.security?.egress?.length) ?? []
  const skillsNoNetwork = skills?.filter((s) => !s.security?.egress?.length) ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Shield className="h-5 w-5 text-sky-400" />
        <h1 className="text-lg font-bold text-slate-100">Security</h1>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardTitle>Active Rate Limit Buckets</CardTitle>
          <CardValue>{rlLoading ? '-' : rateLimit?.active_buckets ?? 0}</CardValue>
        </Card>
        <Card>
          <CardTitle>Rate (req/sec)</CardTitle>
          <CardValue>{rlLoading ? '-' : rateLimit?.config?.rate ?? '-'}</CardValue>
        </Card>
        <Card>
          <CardTitle>Burst Limit</CardTitle>
          <CardValue>{rlLoading ? '-' : rateLimit?.config?.burst ?? '-'}</CardValue>
        </Card>
      </div>

      {/* Egress Posture */}
      <div className="flex items-center gap-2">
        <Globe className="h-4 w-4 text-sky-400" />
        <h2 className="text-sm font-medium text-slate-200">Egress Posture</h2>
      </div>

      {skillsLoading ? (
        <Card><p className="text-sm text-slate-500">Loading...</p></Card>
      ) : !skills?.length ? (
        <Card>
          <p className="text-sm text-slate-500">No skills loaded. Install skills to see their network permissions here.</p>
        </Card>
      ) : (
        <Card className="p-0 overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-slate-700 text-left text-xs uppercase tracking-wider text-slate-500">
                <th className="px-4 py-2">Skill</th>
                <th className="px-4 py-2">Network Mode</th>
                <th className="px-4 py-2">Allowed Egress</th>
                <th className="px-4 py-2">Resources</th>
              </tr>
            </thead>
            <tbody>
              {skillsWithEgress.map((s) => (
                <tr key={s.name} className="border-b border-slate-700/50 text-sm">
                  <td className="px-4 py-2">
                    <div className="font-medium text-slate-200">{s.name}</div>
                    <div className="text-xs text-slate-500">v{s.version}</div>
                  </td>
                  <td className="px-4 py-2">
                    <Badge variant="warning">allowlist</Badge>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex flex-wrap gap-1">
                      {s.security!.egress.map((host) => (
                        <span key={host} className="rounded bg-slate-900 px-2 py-0.5 font-mono text-xs text-sky-400">
                          {host}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-2 text-xs text-slate-400">
                    {s.security!.max_memory && <span className="mr-2">mem: {s.security!.max_memory}</span>}
                    {s.security!.max_cpu && <span className="mr-2">cpu: {s.security!.max_cpu}</span>}
                    {s.security!.timeout && <span>timeout: {s.security!.timeout}</span>}
                  </td>
                </tr>
              ))}
              {skillsNoNetwork.map((s) => (
                <tr key={s.name} className="border-b border-slate-700/50 text-sm">
                  <td className="px-4 py-2">
                    <div className="font-medium text-slate-200">{s.name}</div>
                    <div className="text-xs text-slate-500">v{s.version}</div>
                  </td>
                  <td className="px-4 py-2">
                    <Badge variant="success">
                      <Lock className="mr-1 inline h-3 w-3" />
                      isolated
                    </Badge>
                  </td>
                  <td className="px-4 py-2 text-xs text-slate-500">No network access</td>
                  <td className="px-4 py-2 text-xs text-slate-400">
                    {s.security?.max_memory && <span className="mr-2">mem: {s.security.max_memory}</span>}
                    {s.security?.max_cpu && <span className="mr-2">cpu: {s.security.max_cpu}</span>}
                    {s.security?.timeout && <span>timeout: {s.security.timeout}</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  )
}
