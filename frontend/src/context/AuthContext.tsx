import { createContext, ReactNode, useContext, useEffect, useMemo, useState } from 'react'
import { api, AuthUser } from '../services/api'

type AuthMode = 'login' | 'register'

type AuthContextValue = {
  currentUser: AuthUser | null
  authInitialized: boolean
  authMode: AuthMode
  authLoading: boolean
  authError: string
  setAuthMode: (mode: AuthMode) => void
  login: (email: string, password: string, tenantID: string) => Promise<void>
  register: (email: string, password: string, displayName: string, tenantID: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null)
  const [authInitialized, setAuthInitialized] = useState(false)
  const [authMode, setAuthMode] = useState<AuthMode>('login')
  const [authLoading, setAuthLoading] = useState(false)
  const [authError, setAuthError] = useState('')

  useEffect(() => {
    let cancelled = false
    api
      .session()
      .then((user) => {
        if (cancelled) return
        setCurrentUser(user)
      })
      .catch(() => {
        if (cancelled) return
        setCurrentUser(null)
      })
      .finally(() => {
        if (cancelled) return
        setAuthInitialized(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  async function login(email: string, password: string, tenantID: string) {
    setAuthLoading(true)
    setAuthError('')
    try {
      const user = await api.login(email, password, tenantID)
      setCurrentUser(user)
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : 'Login failed')
      throw error
    } finally {
      setAuthLoading(false)
    }
  }

  async function register(email: string, password: string, displayName: string, tenantID: string) {
    setAuthLoading(true)
    setAuthError('')
    try {
      await api.register(email, password, displayName, tenantID)
      setAuthMode('login')
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : 'Registration failed')
      throw error
    } finally {
      setAuthLoading(false)
    }
  }

  async function logout() {
    try {
      await api.logout(currentUser?.session_id)
    } catch (error) {
      console.error('logout revoke failed', error)
    } finally {
      setCurrentUser(null)
      setAuthMode('login')
    }
  }

  const value = useMemo<AuthContextValue>(
    () => ({
      currentUser,
      authInitialized,
      authMode,
      authLoading,
      authError,
      setAuthMode,
      login,
      register,
      logout,
    }),
    [currentUser, authInitialized, authMode, authLoading, authError],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
