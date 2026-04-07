import { useState } from 'react'
import { useAuditLog, useAcknowledgeAudit } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { formatTime, truncateHash } from '@/lib/utils'
import { FileCheck, ChevronDown, ChevronRight, ChevronLeft, Check } from 'lucide-react'

const PAGE_SIZE = 25

export function AuditPage() {
  const [mismatchOnly, setMismatchOnly] = useState(false)
  const [page, setPage] = useState(0)
  const [expanded, setExpanded] = useState<number | null>(null)
  const { data: audit, isLoading } = useAuditLog({
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
    mismatch: mismatchOnly || undefined,
  })
  const acknowledge = useAcknowledgeAudit()

  const totalPages = Math.ceil((audit?.total ?? 0) / PAGE_SIZE)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FileCheck className="h-5 w-5 text-sky-400" />
          <h1 className="text-lg font-bold text-slate-100">Audit Log</h1>
          {audit && <span className="text-sm text-slate-500">({audit.total} total)</span>}
          {(audit?.unacknowledged_mismatches ?? 0) > 0 && (
            <Badge variant="danger">{audit!.unacknowledged_mismatches} unacknowledged</Badge>
          )}
        </div>
        <label className="flex items-center gap-2 text-sm text-slate-400">
          <input
            type="checkbox"
            checked={mismatchOnly}
            onChange={(e) => { setMismatchOnly(e.target.checked); setPage(0) }}
            className="rounded border-slate-600"
          />
          Mismatches only
        </label>
      </div>

      <Card className="p-0 overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-sm text-slate-500">Loading...</p>
        ) : !audit?.entries?.length ? (
          <p className="p-6 text-sm text-slate-500">
            {mismatchOnly
              ? 'No hash mismatches found.'
              : 'No audit entries. Actions will appear here once skills are invoked.'}
          </p>
        ) : (
          <div className="divide-y divide-slate-700/50">
            {audit.entries.map((entry, i) => {
              const isMismatch = entry.actual_hash && entry.actual_hash !== entry.decision.hash
              const isExpanded = expanded === i
              const globalIndex = page * PAGE_SIZE + i

              return (
                <div key={i}>
                  <button
                    onClick={() => setExpanded(isExpanded ? null : i)}
                    className={`flex w-full items-center justify-between px-4 py-3 text-left text-xs transition-colors ${
                      isMismatch
                        ? 'bg-red-500/10 hover:bg-red-500/15'
                        : 'hover:bg-slate-800/50'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      {isExpanded ? <ChevronDown className="h-3 w-3 text-slate-500" /> : <ChevronRight className="h-3 w-3 text-slate-500" />}
                      <Badge variant={entry.decision.allowed ? 'success' : 'danger'}>
                        {entry.decision.allowed ? 'ALLOW' : 'DENY'}
                      </Badge>
                      <span className="font-medium text-slate-300">{entry.action.skill_name}</span>
                      <span className="text-slate-500">{entry.action.action_type}</span>
                      <span className="hidden text-slate-600 md:inline">→ {entry.action.target}</span>
                    </div>
                    <div className="flex items-center gap-3">
                      {isMismatch && !entry.acknowledged && (
                        <Badge variant="danger">MISMATCH</Badge>
                      )}
                      {entry.acknowledged && (
                        <Badge variant="default">ACK</Badge>
                      )}
                      <span className="font-mono text-slate-600">{truncateHash(entry.decision.hash, 8)}</span>
                      <span className="text-slate-600">{formatTime(entry.timestamp)}</span>
                    </div>
                  </button>

                  {isExpanded && (
                    <div className={`border-t px-4 py-4 text-xs ${isMismatch ? 'border-red-500/10 bg-red-500/5' : 'border-slate-700/30 bg-slate-900/50'}`}>
                      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                        <div>
                          <div className="mb-2 font-medium text-slate-400">Action at Execution</div>
                          <div className="rounded-lg bg-slate-950 p-3">
                            <div className="space-y-1">
                              <div><span className="text-slate-500">skill:</span> <span className="text-slate-200">{entry.action.skill_name} {entry.action.skill_version}</span></div>
                              <div><span className="text-slate-500">type:</span> <span className="text-slate-200">{entry.action.action_type}</span></div>
                              <div><span className="text-slate-500">target:</span> <span className={`${isMismatch && entry.approved_action && entry.action.target !== entry.approved_action.target ? 'text-red-400 font-bold' : 'text-slate-200'}`}>{entry.action.target}</span></div>
                              {entry.action.payload && (
                                <div><span className="text-slate-500">payload:</span> <span className={`${isMismatch && entry.approved_action && entry.action.payload !== entry.approved_action.payload ? 'text-red-400 font-bold' : 'text-slate-200'}`}>{entry.action.payload}</span></div>
                              )}
                            </div>
                          </div>
                          {entry.actual_hash && (
                            <div className="mt-2 font-mono text-slate-600">Hash: {entry.actual_hash}</div>
                          )}
                        </div>
                        {entry.approved_action && (
                          <div>
                            <div className="mb-2 font-medium text-slate-400">Action at Approval</div>
                            <div className="rounded-lg bg-slate-950 p-3">
                              <div className="space-y-1">
                                <div><span className="text-slate-500">skill:</span> <span className="text-slate-200">{entry.approved_action.skill_name} {entry.approved_action.skill_version}</span></div>
                                <div><span className="text-slate-500">type:</span> <span className="text-slate-200">{entry.approved_action.action_type}</span></div>
                                <div><span className="text-slate-500">target:</span> <span className="text-slate-200">{entry.approved_action.target}</span></div>
                                {entry.approved_action.payload && (
                                  <div><span className="text-slate-500">payload:</span> <span className="text-slate-200">{entry.approved_action.payload}</span></div>
                                )}
                              </div>
                            </div>
                            <div className="mt-2 font-mono text-slate-600">Hash: {entry.decision.hash}</div>
                          </div>
                        )}
                      </div>
                      <div className="mt-3 flex items-center justify-between border-t border-slate-700/30 pt-3">
                        <span className="text-slate-500">Reason: {entry.decision.reason}</span>
                        {isMismatch && !entry.acknowledged && (
                          <button
                            onClick={(e) => { e.stopPropagation(); acknowledge.mutate(globalIndex) }}
                            className="flex items-center gap-1 rounded-lg border border-slate-600 px-3 py-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
                          >
                            <Check className="h-3 w-3" />
                            Acknowledge
                          </button>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </Card>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <button
            onClick={() => setPage(Math.max(0, page - 1))}
            disabled={page === 0}
            className="flex items-center gap-1 rounded-lg border border-slate-700 px-3 py-1.5 text-sm text-slate-400 hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
          >
            <ChevronLeft className="h-4 w-4" />
            Previous
          </button>
          <span className="text-sm text-slate-500">
            Page {page + 1} of {totalPages}
          </span>
          <button
            onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
            disabled={page >= totalPages - 1}
            className="flex items-center gap-1 rounded-lg border border-slate-700 px-3 py-1.5 text-sm text-slate-400 hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed"
          >
            Next
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  )
}
