import { useEffect, useState } from 'react'
import { api, DashboardData, Role } from '../services/api'

export function useDashboard(role: Role, enabled: boolean) {
  const [dashboard, setDashboard] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    setLoading(true)
    setError('')
    api
      .getDashboard(role)
      .then((data) => {
        if (!cancelled) setDashboard(data)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load dashboard')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [role, enabled])

  return { dashboard, loading, error, setDashboard }
}
