import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { Role } from '../services/api'

export function ProtectedRoute({ allowedRoles }: { allowedRoles: Role[] }) {
  const { currentUser, authInitialized } = useAuth()
  if (!authInitialized) return null
  if (!currentUser) return <Navigate to="/login" replace />
  if (!allowedRoles.includes(currentUser.role) && currentUser.role !== 'admin') return <Navigate to="/dashboard" replace />
  return <Outlet />
}
