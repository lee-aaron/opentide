import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { useAuthStore } from '@/store/auth'

export function Layout() {
  const demo = useAuthStore((s) => s.demo)

  return (
    <div className="flex h-screen bg-slate-950">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        {demo && (
          <div className="flex items-center justify-center bg-amber-500/10 px-4 py-1.5 text-xs font-medium text-amber-400 border-b border-amber-500/20">
            DEMO MODE — approvals are auto-granted, security controls relaxed
          </div>
        )}
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
