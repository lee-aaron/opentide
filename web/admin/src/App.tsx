import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuthStore } from '@/store/auth'
import { api } from '@/api/client'
import { Layout } from '@/components/layout/Layout'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { TenantsPage } from '@/pages/TenantsPage'
import { SkillsPage } from '@/pages/SkillsPage'
import { AuditPage } from '@/pages/AuditPage'
import { ApprovalsPage } from '@/pages/ApprovalsPage'
import { SecurityPage } from '@/pages/SecurityPage'
import { SettingsPage } from '@/pages/SettingsPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 10_000,
    },
  },
})

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { authenticated, loading } = useAuthStore()

  useEffect(() => {
    api.me()
      .then((res) => useAuthStore.getState().setAuth(res.authenticated, res.demo))
      .catch(() => useAuthStore.getState().setAuth(false))
  }, [])

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-950">
        <div className="text-sm text-slate-500">Loading...</div>
      </div>
    )
  }

  if (!authenticated) {
    return <LoginPage />
  }

  return <>{children}</>
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route
            path="/admin/*"
            element={
              <AuthGuard>
                <Routes>
                  <Route element={<Layout />}>
                    <Route index element={<DashboardPage />} />
                    <Route path="tenants" element={<TenantsPage />} />
                    <Route path="skills" element={<SkillsPage />} />
                    <Route path="audit" element={<AuditPage />} />
                    <Route path="approvals" element={<ApprovalsPage />} />
                    <Route path="security" element={<SecurityPage />} />
                    <Route path="settings" element={<SettingsPage />} />
                  </Route>
                </Routes>
              </AuthGuard>
            }
          />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
