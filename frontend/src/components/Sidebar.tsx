import { NavLink } from 'react-router-dom'
import { Role } from '../services/api'

type WorkspaceConfig = {
  role: Role
  label: string
  tagline: string
  accent: string
}

type SidebarProps = {
  role: Role
  holdCountdown: string
  email: string
  tenantID: string
  canAccessReports: boolean
  canAccessAdmin: boolean
  onLogout: () => void
}

export function Sidebar({ role, holdCountdown, email, tenantID, canAccessReports, canAccessAdmin, onLogout }: SidebarProps) {
  const roles: WorkspaceConfig[] = [
    { role: 'admin', label: 'Admin', tagline: 'Platform control', accent: 'azure' },
    { role: 'coordinator', label: 'Coordinator', tagline: 'Scheduling + content', accent: 'mint' },
    { role: 'instructor', label: 'Instructor', tagline: 'Delivery workspace', accent: 'gold' },
    { role: 'learner', label: 'Learner', tagline: 'Catalog + sessions', accent: 'violet' },
  ]
  const roleConfig = roles.find((item) => item.role === role) ?? roles[0]
  const routeItems: Array<{ to: string; label: string; allowed: boolean }> = [
    { to: '/dashboard', label: 'Dashboard', allowed: true },
    { to: '/bookings', label: 'Bookings', allowed: role === 'admin' || role === 'coordinator' || role === 'learner' },
    { to: '/content', label: 'Content', allowed: true },
    { to: '/schedule', label: 'Schedule', allowed: role === 'admin' || role === 'coordinator' },
    { to: '/tasks', label: 'Tasks', allowed: role === 'admin' || role === 'coordinator' || role === 'instructor' },
    { to: '/reports', label: 'Reports', allowed: canAccessReports },
    { to: '/admin', label: 'Admin', allowed: canAccessAdmin },
    { to: '/learner', label: 'Learner', allowed: role === 'learner' },
  ]
  return (
    <aside className="sidebar glass">
      <div>
        <p className="eyebrow">TrainingOps</p>
        <h1>TrainingOps studio</h1>
        <p className="sidebar-copy">API-driven dashboard with authenticated tenant context.</p>
        <p className="workspace-note">
          Signed in as {email} ({tenantID})
        </p>
      </div>
      <div className="role-list">
        <div className="role-pill active">
          <span>{roleConfig.label}</span>
          <small>{roleConfig.tagline}</small>
        </div>
      </div>
      <nav className="workspace-nav" aria-label="Role navigation">
        <p className="eyebrow">Workspace Menu</p>
        <div className="menu-list">
          {routeItems.filter((item) => item.allowed).map((item) => (
            <NavLink className="menu-item" key={item.to} to={item.to}>
              {item.label}
            </NavLink>
          ))}
        </div>
      </nav>
      <div className="sidebar-footer">
        <div>
          <span className="meta-label">Current role</span>
          <strong>{roleConfig.label}</strong>
        </div>
        <div>
          <span className="meta-label">Hold timer</span>
          <strong>{holdCountdown}</strong>
        </div>
      </div>
      <button className="btn-secondary" onClick={() => void onLogout()}>
        Log out
      </button>
    </aside>
  )
}
