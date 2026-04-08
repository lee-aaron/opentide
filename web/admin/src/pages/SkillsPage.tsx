import { useSkills, useToggleSkill } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Puzzle, Globe, Lock } from 'lucide-react'

function SkillToggle({ toolName, enabled }: { toolName: string; enabled: boolean }) {
  const toggle = useToggleSkill()

  return (
    <button
      onClick={() => toggle.mutate({ toolName, enabled: !enabled })}
      disabled={toggle.isPending}
      className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors ${
        enabled ? 'bg-sky-500' : 'bg-slate-600'
      } ${toggle.isPending ? 'opacity-50' : ''}`}
      title={enabled ? 'Click to disable' : 'Click to enable'}
    >
      <span
        className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform mt-0.5 ${
          enabled ? 'translate-x-4 ml-0.5' : 'translate-x-0.5'
        }`}
      />
    </button>
  )
}

export function SkillsPage() {
  const { data: skills, isLoading } = useSkills()

  const enabledCount = skills?.filter((s) => s.enabled).length ?? 0
  const totalCount = skills?.length ?? 0

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Puzzle className="h-5 w-5 text-sky-400" />
        <h1 className="text-lg font-bold text-slate-100">Skills</h1>
        {skills && <span className="text-sm text-slate-500">({enabledCount}/{totalCount} enabled)</span>}
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
            <Card key={s.name} className={!s.enabled ? 'opacity-50' : ''}>
              <div className="mb-2 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <SkillToggle toolName={s.tool_name} enabled={s.enabled} />
                  <span className="font-medium text-slate-200">{s.name}</span>
                </div>
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
