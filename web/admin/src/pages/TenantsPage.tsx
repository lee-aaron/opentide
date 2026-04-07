import { useState } from 'react'
import { useTenants, useCreateTenant, useDeleteTenant } from '@/api/hooks'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Users, Plus, Trash2 } from 'lucide-react'

export function TenantsPage() {
  const { data: tenants, isLoading } = useTenants()
  const createTenant = useCreateTenant()
  const deleteTenant = useDeleteTenant()
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ id: '', name: '', platform: 'discord', platform_id: '' })

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    createTenant.mutate(form, {
      onSuccess: () => {
        setShowForm(false)
        setForm({ id: '', name: '', platform: 'discord', platform_id: '' })
      },
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Users className="h-5 w-5 text-sky-400" />
          <h1 className="text-lg font-bold text-slate-100">Tenants</h1>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-1 rounded-lg bg-sky-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-sky-600"
        >
          <Plus className="h-4 w-4" />
          Add Tenant
        </button>
      </div>

      {showForm && (
        <Card>
          <form onSubmit={handleCreate} className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <input
              placeholder="ID (e.g. team-alpha)"
              value={form.id}
              onChange={(e) => setForm({ ...form, id: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
              required
            />
            <input
              placeholder="Name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
              required
            />
            <select
              value={form.platform}
              onChange={(e) => setForm({ ...form, platform: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 focus:border-sky-500 focus:outline-none"
            >
              <option value="discord">Discord</option>
              <option value="slack">Slack</option>
              <option value="telegram">Telegram</option>
            </select>
            <input
              placeholder="Platform ID (guild/workspace ID)"
              value={form.platform_id}
              onChange={(e) => setForm({ ...form, platform_id: e.target.value })}
              className="rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none"
            />
            <div className="sm:col-span-2 flex gap-2">
              <button type="submit" className="rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white hover:bg-sky-600">
                Create
              </button>
              <button type="button" onClick={() => setShowForm(false)} className="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-400 hover:bg-slate-800">
                Cancel
              </button>
            </div>
            {createTenant.error && (
              <p className="sm:col-span-2 text-sm text-red-400">{(createTenant.error as Error).message}</p>
            )}
          </form>
        </Card>
      )}

      <Card>
        {isLoading ? (
          <p className="text-sm text-slate-500">Loading...</p>
        ) : !tenants?.length ? (
          <p className="text-sm text-slate-500">No tenants configured. Click "Add Tenant" to create one.</p>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-slate-700 text-left text-xs uppercase tracking-wider text-slate-500">
                <th className="px-3 py-2">ID</th>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">Platform</th>
                <th className="px-3 py-2">Plan</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {tenants.map((t) => (
                <tr key={t.id} className="border-b border-slate-700/50 text-sm">
                  <td className="px-3 py-2 font-mono text-slate-300">{t.id}</td>
                  <td className="px-3 py-2 text-slate-200">{t.name}</td>
                  <td className="px-3 py-2 text-slate-400">{t.platform}</td>
                  <td className="px-3 py-2">
                    <Badge variant={t.plan?.name === 'pro' ? 'success' : 'info'}>
                      {t.plan?.name || 'free'}
                    </Badge>
                  </td>
                  <td className="px-3 py-2">
                    <button
                      onClick={() => deleteTenant.mutate(t.id)}
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
