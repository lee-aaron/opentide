import { useEffect, useState } from 'react'
import { Shield } from 'lucide-react'
import { api } from '@/api/client'
import { useAuthStore } from '@/store/auth'

export function LoginPage() {
  const [secret, setSecret] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [authConfig, setAuthConfig] = useState<{
    google_enabled: boolean
    secret_enabled: boolean
  } | null>(null)
  const setAuth = useAuthStore((s) => s.setAuth)

  useEffect(() => {
    api.authConfig()
      .then(setAuthConfig)
      .catch(() => setAuthConfig({ google_enabled: false, secret_enabled: true }))

    // Check for OAuth error in URL
    const params = new URLSearchParams(window.location.search)
    const oauthError = params.get('error')
    if (oauthError) {
      const messages: Record<string, string> = {
        unauthorized_email: 'Your email is not authorized. Contact an admin.',
        unverified_email: 'Your Google email is not verified.',
        invalid_state: 'Login session expired. Please try again.',
        token_exchange_failed: 'Failed to authenticate with Google. Please try again.',
      }
      setError(messages[oauthError] || `Login failed: ${oauthError}`)
      // Clean URL
      window.history.replaceState({}, '', '/admin')
    }
  }, [])

  async function handleSecretSubmit(e: React.FormEvent) {
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

  function handleGoogleLogin() {
    window.location.href = '/admin/api/auth/google'
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <Shield className="mx-auto mb-4 h-12 w-12 text-sky-400" />
          <h1 className="text-2xl font-bold text-slate-100">OpenTide Admin</h1>
          <p className="mt-1 text-sm text-slate-500">Security Control Plane</p>
        </div>
        <div className="rounded-xl border border-slate-700 bg-slate-800 p-6">
          {error && (
            <p className="mb-4 rounded-lg bg-red-900/30 border border-red-800 px-3 py-2 text-sm text-red-400">{error}</p>
          )}

          {authConfig?.google_enabled && (
            <>
              <button
                onClick={handleGoogleLogin}
                className="flex w-full items-center justify-center gap-2 rounded-lg border border-slate-600 bg-slate-900 px-4 py-2.5 text-sm font-medium text-slate-200 transition-colors hover:bg-slate-700"
              >
                <svg className="h-4 w-4" viewBox="0 0 24 24">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                Sign in with Google
              </button>
              {authConfig?.secret_enabled && (
                <div className="my-4 flex items-center gap-3">
                  <div className="h-px flex-1 bg-slate-700" />
                  <span className="text-xs text-slate-500">or</span>
                  <div className="h-px flex-1 bg-slate-700" />
                </div>
              )}
            </>
          )}

          {authConfig?.secret_enabled && (
            <form onSubmit={handleSecretSubmit}>
              <label className="mb-2 block text-sm font-medium text-slate-300">
                Admin Secret
              </label>
              <input
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder="Enter OPENTIDE_ADMIN_SECRET"
                className="mb-4 w-full rounded-lg border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-slate-200 placeholder-slate-500 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
                autoFocus={!authConfig?.google_enabled}
              />
              <button
                type="submit"
                disabled={loading || !secret}
                className="w-full rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-sky-600 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {loading ? 'Authenticating...' : 'Sign In with Secret'}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
