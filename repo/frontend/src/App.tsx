import { Outlet } from 'react-router-dom'
import { Sidebar } from './components/Sidebar'
import { useAuth } from './context/AuthContext'
import { useDashboard } from './hooks/useDashboard'

export default function AppLayout() {
  const { currentUser, logout } = useAuth()
  const role = currentUser?.role ?? 'learner'
  const { dashboard } = useDashboard(role, Boolean(currentUser))
  const holdCountdown = dashboard?.countdownEnd
    ? (() => {
        const remaining = Math.max(0, new Date(dashboard.countdownEnd).getTime() - Date.now())
        const minutes = Math.floor(remaining / 60000)
        const seconds = Math.floor((remaining % 60000) / 1000)
        return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
      })()
    : 'No hold timer'

  if (!currentUser) return <Outlet />

  return (
    <main className={`app-shell theme-${role}`}>
      <Sidebar
        role={role}
        holdCountdown={holdCountdown}
        email={currentUser.email}
        tenantID={currentUser.tenant_id}
        canAccessReports={role === 'admin' || role === 'coordinator'}
        canAccessAdmin={role === 'admin'}
        onLogout={logout}
      />
      <section className="dashboard">
        <Outlet />
      </section>
    </main>
  )
}
