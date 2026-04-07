import { useState } from 'react'
import { useProviders, useRoutes, useCreateRoute, useDeleteRoute, useTestRoute, useSkills, useSecrets, useSetSecret, useDeleteSecret } from '@/api/hooks'
import type { ProviderRoute } from '@/api/types'

function HealthBadge({ healthy }: { healthy: boolean }) {
  return (
    <span className={`inline-flex h-2 w-2 rounded-full ${healthy ? 'bg-green-400' : 'bg-red-400'}`} />
  )
}

function ProviderCards() {
  const { data: providers, isLoading } = useProviders()

  if (isLoading) return <div className="text-sm text-slate-500">Loading providers...</div>
  if (!providers?.length) return <div className="text-sm text-slate-500">No providers configured.</div>

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {providers.map((p) => (
        <div key={p.name} className="rounded-lg border border-slate-700 bg-slate-800/50 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <HealthBadge healthy={p.healthy} />
              <span className="text-sm font-medium text-slate-200">{p.name}</span>
            </div>
            {p.is_default && (
              <span className="rounded bg-sky-500/20 px-2 py-0.5 text-xs text-sky-400">default</span>
            )}
          </div>
          <div className="mt-2 text-xs text-slate-500">{p.model || 'default model'}</div>
        </div>
      ))}
    </div>
  )
}

function RouteTable() {
  const { data: routes, isLoading } = useRoutes()
  const deleteRoute = useDeleteRoute()
  const [showForm, setShowForm] = useState(false)

  if (isLoading) return <div className="text-sm text-slate-500">Loading routes...</div>

  return (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-slate-300">Routes</h3>
        <button
          onClick={() => setShowForm(!showForm)}
          className="rounded bg-sky-600 px-3 py-1 text-xs text-white hover:bg-sky-500"
        >
          {showForm ? 'Cancel' : 'Add Route'}
        </button>
      </div>

      {showForm && <RouteForm onDone={() => setShowForm(false)} />}

      {!routes?.length ? (
        <div className="rounded border border-slate-700 bg-slate-800/30 p-6 text-center text-sm text-slate-500">
          No routes configured. All messages use the default provider.
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-slate-700">
          <table className="w-full text-sm">
            <thead className="bg-slate-800/50">
              <tr className="text-left text-xs text-slate-400">
                <th className="px-4 py-2">Priority</th>
                <th className="px-4 py-2">Channel</th>
                <th className="px-4 py-2">Provider</th>
                <th className="px-4 py-2">Model</th>
                <th className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-700">
              {routes.map((route, i) => (
                <tr key={i} className="text-slate-300 hover:bg-slate-800/30">
                  <td className="px-4 py-2 font-mono text-xs">{route.priority}</td>
                  <td className="px-4 py-2">{route.channel_id || '*'}</td>
                  <td className="px-4 py-2">{route.provider}</td>
                  <td className="px-4 py-2 text-slate-500">{route.model || 'default'}</td>
                  <td className="px-4 py-2 text-right">
                    <button
                      onClick={() => deleteRoute.mutate(i)}
                      className="text-xs text-red-400 hover:text-red-300"
                      disabled={deleteRoute.isPending}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function RouteForm({ onDone }: { onDone: () => void }) {
  const createRoute = useCreateRoute()
  const { data: providers } = useProviders()
  const [form, setForm] = useState<Partial<ProviderRoute>>({
    channel_id: '',
    provider: '',
    model: '',
    priority: 1,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.provider) return
    createRoute.mutate(
      {
        channel_id: form.channel_id || '*',
        provider: form.provider,
        model: form.model,
        priority: form.priority || 1,
      },
      { onSuccess: onDone }
    )
  }

  return (
    <form onSubmit={handleSubmit} className="mb-4 rounded-lg border border-slate-700 bg-slate-800/50 p-4">
      <div className="grid grid-cols-4 gap-3">
        <div>
          <label className="mb-1 block text-xs text-slate-400">Channel ID</label>
          <input
            type="text"
            value={form.channel_id}
            onChange={(e) => setForm({ ...form, channel_id: e.target.value })}
            placeholder="* (all)"
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-slate-400">Provider</label>
          <select
            value={form.provider}
            onChange={(e) => setForm({ ...form, provider: e.target.value })}
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          >
            <option value="">Select...</option>
            {providers?.map((p) => (
              <option key={p.name} value={p.name}>{p.name}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs text-slate-400">Model (optional)</label>
          <input
            type="text"
            value={form.model}
            onChange={(e) => setForm({ ...form, model: e.target.value })}
            placeholder="default"
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-slate-400">Priority</label>
          <input
            type="number"
            value={form.priority}
            onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          />
        </div>
      </div>
      <div className="mt-3 flex justify-end">
        <button
          type="submit"
          disabled={!form.provider || createRoute.isPending}
          className="rounded bg-sky-600 px-4 py-1 text-xs text-white hover:bg-sky-500 disabled:opacity-50"
        >
          Create Route
        </button>
      </div>
    </form>
  )
}

function TestRoutePanel() {
  const [userID, setUserID] = useState('')
  const [channelID, setChannelID] = useState('')
  const testRoute = useTestRoute()
  const [open, setOpen] = useState(false)

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="text-xs text-sky-400 hover:text-sky-300"
      >
        Test route resolution...
      </button>
    )
  }

  return (
    <div className="rounded-lg border border-slate-700 bg-slate-800/50 p-4">
      <h3 className="mb-3 text-sm font-medium text-slate-300">Test Route Resolution</h3>
      <div className="grid grid-cols-3 gap-3">
        <div>
          <label className="mb-1 block text-xs text-slate-400">Channel ID</label>
          <input
            type="text"
            value={channelID}
            onChange={(e) => setChannelID(e.target.value)}
            placeholder="engineering"
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs text-slate-400">User ID (optional)</label>
          <input
            type="text"
            value={userID}
            onChange={(e) => setUserID(e.target.value)}
            placeholder="user123"
            className="w-full rounded border border-slate-600 bg-slate-900 px-2 py-1 text-sm text-slate-200"
          />
        </div>
        <div className="flex items-end gap-2">
          <button
            onClick={() => testRoute.mutate({ userID, channelID })}
            disabled={testRoute.isPending}
            className="rounded bg-sky-600 px-4 py-1 text-xs text-white hover:bg-sky-500 disabled:opacity-50"
          >
            Test
          </button>
          <button
            onClick={() => setOpen(false)}
            className="rounded px-3 py-1 text-xs text-slate-400 hover:text-slate-200"
          >
            Close
          </button>
        </div>
      </div>

      {testRoute.data && (
        <div className="mt-3 rounded border border-slate-700 bg-slate-900 p-3 text-xs text-slate-300">
          {testRoute.data.resolved ? (
            <>
              <div><span className="text-slate-500">Provider:</span> {testRoute.data.provider}</div>
              <div><span className="text-slate-500">Model:</span> {testRoute.data.model}</div>
              {testRoute.data.has_override && (
                <div className="mt-1 text-amber-400">User override active: {testRoute.data.override_name}</div>
              )}
            </>
          ) : (
            <div className="text-red-400">No provider resolved for this combination.</div>
          )}
        </div>
      )}
    </div>
  )
}

const PROVIDER_LABELS: Record<string, { label: string; placeholder: string; hint: string }> = {
  anthropic: { label: 'Anthropic', placeholder: 'sk-ant-...', hint: 'Get your key at console.anthropic.com' },
  openai: { label: 'OpenAI', placeholder: 'sk-...', hint: 'Get your key at platform.openai.com' },
  gradient: { label: 'DO Gradient', placeholder: 'dop_v1_...', hint: 'DO Control Panel > Serverless Inference > Model Access Keys' },
}

function ApiKeyManager() {
  const { data: secretsList, isLoading } = useSecrets()
  const setSecret = useSetSecret()
  const deleteSecretMut = useDeleteSecret()
  const [editing, setEditing] = useState<string | null>(null)
  const [apiKey, setApiKey] = useState('')
  const [error, setError] = useState('')

  if (isLoading) return <div className="text-sm text-slate-500">Loading API keys...</div>

  const secretsMap = new Map((secretsList ?? []).map((s) => [s.provider, s]))

  const handleSave = (provider: string) => {
    if (!apiKey.trim()) return
    setError('')
    setSecret.mutate(
      { provider, api_key: apiKey.trim() },
      {
        onSuccess: () => {
          setEditing(null)
          setApiKey('')
        },
        onError: (err) => setError(err instanceof Error ? err.message : 'Failed to save'),
      }
    )
  }

  const handleDelete = (provider: string) => {
    deleteSecretMut.mutate(provider)
  }

  return (
    <div>
      <h3 className="mb-3 text-sm font-medium text-slate-300">API Keys</h3>
      <div className="space-y-3">
        {Object.entries(PROVIDER_LABELS).map(([name, info]) => {
          const secret = secretsMap.get(name)
          const isConfigured = secret?.configured
          const isEnv = secret?.source === 'env'
          const isEditing = editing === name

          return (
            <div key={name} className="rounded-lg border border-slate-700 bg-slate-800/50 p-4">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-sm font-medium text-slate-200">{info.label}</span>
                  <span className="ml-2 text-xs text-slate-500">{info.hint}</span>
                </div>
                <div className="flex items-center gap-2">
                  {isConfigured && isEnv && (
                    <span className="rounded bg-slate-600/50 px-2 py-0.5 text-xs text-slate-300">
                      via env ****{secret.last4}
                    </span>
                  )}
                  {isConfigured && !isEnv && (
                    <>
                      <span className="rounded bg-green-500/20 px-2 py-0.5 text-xs text-green-400">
                        configured ****{secret.last4}
                      </span>
                      <button
                        onClick={() => handleDelete(name)}
                        disabled={deleteSecretMut.isPending}
                        className="text-xs text-red-400 hover:text-red-300"
                      >
                        Remove
                      </button>
                    </>
                  )}
                  {!isConfigured && !isEditing && (
                    <button
                      onClick={() => { setEditing(name); setApiKey(''); setError('') }}
                      className="rounded bg-sky-600 px-3 py-1 text-xs text-white hover:bg-sky-500"
                    >
                      Add Key
                    </button>
                  )}
                </div>
              </div>

              {isEditing && (
                <div className="mt-3">
                  <div className="flex gap-2">
                    <input
                      type="password"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={info.placeholder}
                      className="flex-1 rounded border border-slate-600 bg-slate-900 px-3 py-1.5 text-sm text-slate-200 placeholder:text-slate-600"
                      autoFocus
                      onKeyDown={(e) => e.key === 'Enter' && handleSave(name)}
                    />
                    <button
                      onClick={() => handleSave(name)}
                      disabled={!apiKey.trim() || setSecret.isPending}
                      className="rounded bg-green-600 px-4 py-1.5 text-xs text-white hover:bg-green-500 disabled:opacity-50"
                    >
                      {setSecret.isPending ? 'Saving...' : 'Save'}
                    </button>
                    <button
                      onClick={() => { setEditing(null); setApiKey(''); setError('') }}
                      className="rounded px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200"
                    >
                      Cancel
                    </button>
                  </div>
                  {error && <div className="mt-2 text-xs text-red-400">{error}</div>}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function SkillDependencies() {
  const { data: skills } = useSkills()
  const { data: providers } = useProviders()
  const [open, setOpen] = useState(false)

  const providerNames = new Set(providers?.map((p) => p.name) ?? [])

  // Skills that require specific providers
  const deps = [
    { skill: 'image-gen', requires: 'openai', key: 'OPENAI_API_KEY' },
    { skill: 'web-search', requires: null, key: null },
    { skill: 'github', requires: null, key: 'GITHUB_TOKEN' },
  ]

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="text-xs text-slate-500 hover:text-slate-300"
      >
        Skill dependencies...
      </button>
    )
  }

  return (
    <div className="rounded-lg border border-slate-700 bg-slate-800/50 p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-slate-300">Skill Dependencies</h3>
        <button onClick={() => setOpen(false)} className="text-xs text-slate-500 hover:text-slate-300">Close</button>
      </div>
      {!skills?.length ? (
        <div className="text-sm text-slate-500">No skills loaded.</div>
      ) : (
        <div className="space-y-1">
          {skills.map((skill) => {
            const dep = deps.find((d) => d.skill === skill.name)
            const missing = dep?.requires && !providerNames.has(dep.requires)
            return (
              <div key={skill.name} className="flex items-center justify-between text-xs">
                <span className="text-slate-300">{skill.name}</span>
                {missing ? (
                  <span className="text-amber-400">Requires {dep.requires} provider ({dep.key})</span>
                ) : (
                  <span className="text-green-400">Ready</span>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

export function ProvidersPage() {
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-slate-100">Providers</h2>
        <p className="text-sm text-slate-500">Manage LLM providers and routing rules.</p>
      </div>

      <ApiKeyManager />
      <ProviderCards />
      <RouteTable />
      <TestRoutePanel />
      <SkillDependencies />
    </div>
  )
}
