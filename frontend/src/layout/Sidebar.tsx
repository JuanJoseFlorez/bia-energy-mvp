import { Gauge, LayoutDashboard, LogOut, Sparkles } from 'lucide-react'
import { NavLink } from 'react-router'

import { useDashboard } from '../api/queries'
import logo from '../assets/logo.png'
import { useAuth } from '../features/auth/authContext'

const NAV = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/meters', label: 'Medidores', icon: Gauge, end: false },
  { to: '/anomalies', label: 'Anomalías IA', icon: Sparkles, end: false },
]

export function Sidebar() {
  const { session, signOut } = useAuth()
  const highPriority = useDashboard().data?.anomalies?.high_priority ?? 0

  return (
    <aside className="flex w-16 shrink-0 flex-col gap-6 border-r border-border bg-surface px-3 py-5 xl:w-60">
      <img src={logo} alt="Bia" className="h-6 self-start object-contain object-left px-2" />
      <nav className="flex flex-col gap-1">
        {NAV.map(({ to, label, icon: Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            title={label}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2 text-sm ${isActive ? 'bg-surface-2 text-text' : 'text-muted hover:bg-surface-2 hover:text-text'}`
            }
          >
            <Icon className="size-4 shrink-0" aria-hidden />
            <span className="hidden flex-1 xl:inline">{label}</span>
            {to === '/anomalies' && highPriority > 0 && (
              <span aria-label={`${highPriority} de alta prioridad`} className="hidden rounded-md border border-danger/40 bg-danger/10 px-1.5 text-xs text-danger xl:inline">
                {highPriority}
              </span>
            )}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex items-center justify-between gap-2 rounded-xl border border-border px-3 py-2">
        <span className="hidden truncate text-sm xl:inline">{session?.user.username}</span>
        <button type="button" onClick={signOut} title="Cerrar sesión" className="flex items-center gap-2 text-sm text-muted hover:text-text">
          <LogOut className="size-4" aria-hidden />
          <span className="hidden xl:inline">Cerrar sesión</span>
        </button>
      </div>
    </aside>
  )
}
