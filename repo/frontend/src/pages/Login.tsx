import { useEffect, useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { api } from '../services/api'

export function LoginPage() {
  const { currentUser, authMode, setAuthMode, authLoading, authError, login, register } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [tenantID, setTenantID] = useState('')
  const [tenantName, setTenantName] = useState('')
  const [tenantSlug, setTenantSlug] = useState('')
  const [adminUsername, setAdminUsername] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [adminPassword, setAdminPassword] = useState('')
  const [needsSetup, setNeedsSetup] = useState(false)
  const [setupMessage, setSetupMessage] = useState('')

  useEffect(() => {
    let cancelled = false
    api
      .setupStatus()
      .then(() => {
        if (cancelled) return
        setNeedsSetup(false)
      })
      .catch((error) => {
        if (cancelled) return
        if (error instanceof Error && error.message.includes('404')) {
          setNeedsSetup(true)
          return
        }
        setSetupMessage(error instanceof Error ? error.message : 'Unable to check setup status')
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (currentUser) return <Navigate to="/dashboard" replace />

  async function submit() {
    if (!tenantID.trim()) return
    if (authMode === 'login') await login(email.trim().toLowerCase(), password, tenantID.trim())
    else await register(email.trim().toLowerCase(), password, displayName.trim(), tenantID.trim())
  }

  async function bootstrapTenant() {
    if (!tenantName.trim() || !adminUsername.trim() || !adminEmail.trim() || !adminPassword.trim()) {
      setSetupMessage('All setup fields are required')
      return
    }
    try {
      const result = await api.bootstrapTenant({
        tenant_name: tenantName.trim(),
        tenant_slug: tenantSlug.trim() || tenantName.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-'),
        admin_username: adminUsername.trim(),
        admin_email: adminEmail.trim().toLowerCase(),
        admin_password: adminPassword,
      })
      setTenantID(result.tenant_slug)
      setNeedsSetup(false)
      setAuthMode('login')
      setSetupMessage('Tenant bootstrap completed. Sign in with the admin account.')
    } catch (error) {
      setSetupMessage(error instanceof Error ? error.message : 'Tenant bootstrap failed')
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-card glass">
        <div className="auth-head">
          <p className="eyebrow">TrainingOps Access</p>
          <h1>Sign in to continue</h1>
        </div>
        {needsSetup ? (
          <article className="panel glass" style={{ marginBottom: 16 }}>
            <div className="panel-header">
              <h3>First-run setup</h3>
              <p>Create the first tenant and administrator account.</p>
            </div>
            <div className="auth-form-grid">
              <label>
                Tenant Name
                <input value={tenantName} onChange={(e) => setTenantName(e.target.value)} />
              </label>
              <label>
                Tenant Slug
                <input value={tenantSlug} onChange={(e) => setTenantSlug(e.target.value)} placeholder="auto-generated if blank" />
              </label>
              <label>
                Admin Username
                <input value={adminUsername} onChange={(e) => setAdminUsername(e.target.value)} />
              </label>
              <label>
                Admin Email
                <input type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} />
              </label>
              <label>
                Admin Password
                <input type="password" value={adminPassword} onChange={(e) => setAdminPassword(e.target.value)} />
              </label>
            </div>
            {setupMessage ? <p className="auth-message">{setupMessage}</p> : null}
            <button className="btn-secondary auth-submit" onClick={bootstrapTenant}>
              Bootstrap tenant
            </button>
          </article>
        ) : null}
        <div className="auth-mode-toggle">
          <button className={authMode === 'login' ? 'active' : ''} onClick={() => setAuthMode('login')}>
            Login
          </button>
          <button className={authMode === 'register' ? 'active' : ''} onClick={() => setAuthMode('register')}>
            Register
          </button>
        </div>
        <div className="auth-form-grid">
          {authMode === 'register' ? (
            <>
              <label>
                Display Name
                <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
              </label>
            </>
          ) : null}
          <label>
            Tenant ID
            <input value={tenantID} onChange={(e) => setTenantID(e.target.value)} placeholder="tenant slug or id" />
          </label>
          <label>
            Email
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </label>
          <label>
            Password
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </label>
        </div>
        {authError ? <p className="auth-message">{authError}</p> : null}
        <button className="btn-primary auth-submit" onClick={submit} disabled={authLoading}>
          {authLoading ? 'Please wait...' : authMode === 'login' ? 'Sign In' : 'Create Account'}
        </button>
      </section>
    </main>
  )
}
