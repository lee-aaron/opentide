import { useState } from 'react'
import { usePolicies, useCreatePolicy, useDeletePolicy } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { truncateHash, formatTime } from '@/lib/utils'
import { Activity, Plus, Trash2 } from 'lucide-react'

export function ApprovalsPage() {
  const { data: policies, isLoading } = usePolicies()
  const createPolicy = useCreatePolicy()
  const deletePolicy = useDeletePolicy()
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({
    skill_name: '',
    action_type: 'network',
    target: '',
    allowed: true,
    reason: '',
  })

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createPolicy.mutate(form, {
      onSuccess: () => {
        setShowForm(false)
        setForm({ skill_name: '', action_type: 'network', target: '', allowed: true, reason: '' })
      },
    })
  }

  function policyKey(p: { scope: { skill_name: string; action_type: string; target: string } }) {
    return `${p.scope.skill_name}|${p.scope.action_type}|${p.scope.target}`
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Activity className="h-5 w-5 text-sky-400" />
          <h1 className="text-lg font-bold text-slate-100">Approval Policies</h1>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-1 rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-600"
        >
          <Plus className="h-4 w-4" />
          Add Policy
        </button>
      </div>

      {showForm && (
        <Card>
          <form onSubmit={handleCreate} className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <input
              placeholder="Skill name"
              value={form.skill_name}
              onChange={(e) => setForm({ ...form, skill_name: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
              required
            />
            <select
              value={form.action_type}
              onChange={(e) => setForm({ ...form, action_type: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 focus:border-sky-500 focus:outline-none"
            >
              <option value="network">Network</option>
              <option value="filesystem">Filesystem</option>
              <option value="shell">Shell</option>
            </select>
            <input
              placeholder="Target (e.g. api.example.com:443)"
              value={form.target}
              onChange={(e) => setForm({ ...form, target: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
            />
            <select
              value={form.allowed ? 'allow' : 'deny'}
              onChange={(e) => setForm({ ...form, allowed: e.target.value === 'allow' })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 focus:border-sky-500 focus:outline-none"
            >
              <option value="allow">Allow</option>
              <option value="deny">Deny</option>
            </select>
            <input
              placeholder="Reason (optional)"
              value={form.reason}
              onChange={(e) => setForm({ ...form, reason: e.target.value })}
              className="sm:col-span-2 rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
            />
            <div className="sm:col-span-2 flex gap-2">
              <button type="submit" className="rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white hover:bg-sky-600">
                Create
              </button>
              <button type="button" onClick={() => setShowForm(false)} className="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-400 hover:bg-slate-800">
                Cancel
              </button>
            </div>
            {createPolicy.error && (
              <p className="sm:col-span-2 text-sm text-red-400">{(createPolicy.error as Error).message}</p>
            )}
          </form>
        </Card>
      )}

      <Card className="p-0 overflow-hidden">
        {isLoading ? (
          <p className="p-6 text-sm text-slate-500">Loading...</p>
        ) : !policies?.length ? (
          <p className="p-6 text-sm text-slate-500">No active policies. Policies appear when skills request approval, or click "Add Policy" to create one manually.</p>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-slate-700 text-left text-xs uppercase tracking-wider text-slate-500">
                <th className="px-4 py-2">Status</th>
                <th className="px-4 py-2">Skill</th>
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Target</th>
                <th className="px-4 py-2">Hash</th>
                <th className="px-4 py-2">Expires</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {policies.map((p, i) => (
                <tr key={i} className="border-b border-slate-700/50 text-sm">
                  <td className="px-4 py-2">
                    <Badge variant={p.allowed ? 'success' : 'warning'}>
                      {p.allowed ? 'ALLOW' : 'DENY'}
                    </Badge>
                  </td>
                  <td className="px-4 py-2 text-slate-300">{p.scope.skill_name}</td>
                  <td className="px-4 py-2 text-slate-400">{p.scope.action_type}</td>
                  <td className="px-4 py-2 font-mono text-xs text-slate-400">{p.scope.target || '-'}</td>
                  <td className="px-4 py-2 font-mono text-xs text-slate-500">{truncateHash(p.hash, 12)}</td>
                  <td className="px-4 py-2 text-xs text-slate-500">{formatTime(p.expires_at)}</td>
                  <td className="px-4 py-2">
                    <button
                      onClick={() => deletePolicy.mutate(policyKey(p))}
                      className="text-slate-500 hover:text-red-400"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  )
}
