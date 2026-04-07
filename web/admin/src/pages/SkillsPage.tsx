import { useSkills } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Puzzle, Globe, Lock } from 'lucide-react'

export function SkillsPage() {
  const { data: skills, isLoading } = useSkills()

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Puzzle className="h-5 w-5 text-sky-400" />
        <h1 className="text-lg font-bold text-slate-100">Skills</h1>
        {skills && <span className="text-sm text-slate-500">({skills.length} loaded)</span>}
      </div>

      {isLoading ? (
        <Card><p className="text-sm text-slate-500">Loading...</p></Card>
      ) : !skills?.length ? (
        <Card>
          <p className="text-sm text-slate-500">
            No skills loaded. Add skill directories to <code className="text-sky-400">skills/</code> with a <code className="text-sky-400">skill.yaml</code> manifest.
          </p>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {skills.map((s) => (
            <Card key={s.name}>
              <div className="mb-2 flex items-center justify-between">
                <span className="font-medium text-slate-200">{s.name}</span>
                <Badge variant="info">{s.version}</Badge>
              </div>
              <p className="mb-3 text-xs text-slate-400">{s.description || 'No description'}</p>
              <div className="mb-3 text-xs text-slate-500">
                Tool: <code className="text-sky-400">{s.tool_name}</code>
              </div>

              {/* Security posture */}
              {s.security && (
                <div className="border-t border-slate-700 pt-3">
                  <div className="mb-2 flex items-center gap-1 text-xs text-slate-500">
                    {s.security.egress?.length ? (
                      <><Globe className="h-3 w-3" /> Network: allowlist</>
                    ) : (
                      <><Lock className="h-3 w-3 text-green-400" /> <span className="text-green-400">No network</span></>
                    )}
                  </div>
                  {s.security.egress?.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {s.security.egress.map((host) => (
                        <span key={host} className="rounded bg-slate-900 px-1.5 py-0.5 font-mono text-xs text-sky-400">
                          {host}
                        </span>
                      ))}
                    </div>
                  )}
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-600">
                    {s.security.filesystem && <span>fs: {s.security.filesystem}</span>}
                    {s.security.max_memory && <span>mem: {s.security.max_memory}</span>}
                    {s.security.max_cpu && <span>cpu: {s.security.max_cpu}</span>}
                    {s.security.timeout && <span>timeout: {s.security.timeout}</span>}
                  </div>
                </div>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
