import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider, useAuth } from './AuthContext'

function Harness() {
  const { currentUser, authError, login, logout } = useAuth()
  return (
    <div>
      <span data-testid="role">{currentUser?.role ?? 'none'}</span>
      <span data-testid="error">{authError}</span>
      <button onClick={() => void login('user@example.com', 'Password123!', 'tenant-1')}>login</button>
      <button onClick={() => void logout()}>logout</button>
    </div>
  )
}

describe('AuthContext', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mockAuthFetch() {
    return vi.fn().mockImplementation(async (url: string) => {
      if (url.includes('/v1/auth/session')) {
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ error: 'session not found' }),
        }
      }
      if (url.includes('/v1/auth/logout')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ status: 'revoked' }),
        }
      }
      return {
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          user_id: 'u-1',
          tenant_id: 'tenant-1',
          email: 'u***@example.com',
          role: 'coordinator',
          status: 'authenticated',
          session_id: 'session-1',
        }),
      }
    })
  }

  it('logs in successfully', async () => {
    vi.stubGlobal('fetch', mockAuthFetch())
    render(
      <AuthProvider>
        <Harness />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByText('login'))

    await waitFor(() => {
      expect(screen.getByTestId('role')).toHaveTextContent('coordinator')
    })
  })

  it('shows login failure error', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (url: string) => {
        if (url.includes('/v1/auth/session')) {
          return {
            ok: false,
            status: 401,
            headers: new Headers({ 'content-type': 'application/json' }),
            json: async () => ({ error: 'session not found' }),
          }
        }
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ error: 'Invalid credentials' }),
        }
      }),
    )

    render(
      <AuthProvider>
        <Harness />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByText('login'))

    await waitFor(() => {
      expect(screen.getByTestId('error')).toHaveTextContent('Invalid credentials')
    })
  })

  it('calls logout revoke endpoint before clearing state', async () => {
    const fetchSpy = vi.fn().mockImplementation(async (url: string) => {
      if (url.includes('/v1/auth/session')) {
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ error: 'session not found' }),
        }
      }
      if (url.includes('/v1/auth/logout')) {
        return {
          ok: true,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ status: 'revoked' }),
        }
      }
      return {
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          user_id: 'u-1',
          tenant_id: 'tenant-1',
          email: 'u***@example.com',
          role: 'coordinator',
          status: 'authenticated',
          session_id: 'session-1',
        }),
      }
    })

    vi.stubGlobal('fetch', fetchSpy)
    render(
      <AuthProvider>
        <Harness />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByText('login'))
    await waitFor(() => expect(screen.getByTestId('role')).toHaveTextContent('coordinator'))
    fireEvent.click(screen.getByText('logout'))

    await waitFor(() => expect(screen.getByTestId('role')).toHaveTextContent('none'))
    expect(fetchSpy).toHaveBeenCalledWith(expect.stringContaining('/v1/auth/logout'), expect.objectContaining({ method: 'POST' }))
  })
})
