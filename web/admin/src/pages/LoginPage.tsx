import { useState } from 'react'
import { Shield } from 'lucide-react'
import { api } from '@/api/client'
import { useAuthStore } from '@/store/auth'

export function LoginPage() {
  const [secret, setSecret] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const setAuth = useAuthStore((s) => s.setAuth)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await api.login(secret)
      setAuth(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <Shield className="mx-auto mb-4 h-12 w-12 text-sky-400" />
          <h1 className="text-2xl font-bold text-slate-100">OpenTide Admin</h1>
          <p className="mt-1 text-sm text-slate-500">Security Control Plane</p>
        </div>
        <form onSubmit={handleSubmit} className="rounded-xl border border-slate-700 bg-slate-800 p-6">
          <label className="mb-2 block text-sm font-medium text-slate-300">
            Admin Secret
          </label>
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            placeholder="Enter OPENTIDE_ADMIN_SECRET"
            className="mb-4 w-full rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
            autoFocus
          />
          {error && (
            <p className="mb-4 text-sm text-red-400">{error}</p>
          )}
          <button
            type="submit"
            disabled={loading || !secret}
            className="w-full rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-sky-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Authenticating...' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}
