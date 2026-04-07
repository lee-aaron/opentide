import { NavLink } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { LayoutDashboard, Shield, Users, Puzzle, FileCheck, Activity, Settings } from 'lucide-react'
import { useAuditLog } from '@/api/hooks'

const navItems = [
  { to: '/admin', icon: LayoutDashboard, label: 'Dashboard', end: true },
  { to: '/admin/security', icon: Shield, label: 'Security' },
  { to: '/admin/audit', icon: FileCheck, label: 'Audit Log' },
  { to: '/admin/approvals', icon: Activity, label: 'Approvals' },
  { to: '/admin/tenants', icon: Users, label: 'Tenants' },
  { to: '/admin/skills', icon: Puzzle, label: 'Skills' },
  { to: '/admin/settings', icon: Settings, label: 'Settings' },
]

export function Sidebar() {
  // Use limit=1 to minimize payload; we only need unacknowledged_mismatches count
  const { data: audit } = useAuditLog({ limit: 1 })
  const mismatches = audit?.unacknowledged_mismatches ?? 0

  return (
    <aside className="flex h-screen w-56 flex-col border-r border-slate-700 bg-slate-900">
      <div className="flex items-center gap-2 border-b border-slate-700 px-4 py-4">
        <Shield className="h-6 w-6 text-sky-400" />
        <div>
          <div className="text-sm font-bold text-slate-100">OpenTide</div>
          <div className="text-xs text-slate-500">Security Control Plane</div>
        </div>
      </div>
      <nav className="flex-1 space-y-1 px-2 py-4">
        {navItems.map(({ to, icon: Icon, label, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-slate-800 text-sky-400'
                  : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
            {label === 'Audit Log' && mismatches > 0 && (
              <span className="ml-auto flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs font-bold text-white">
                {mismatches}
              </span>
            )}
          </NavLink>
        ))}
      </nav>
      <div className="border-t border-slate-700 px-4 py-3">
        <div className="text-xs text-slate-600">v0.1.0</div>
      </div>
    </aside>
  )
}
