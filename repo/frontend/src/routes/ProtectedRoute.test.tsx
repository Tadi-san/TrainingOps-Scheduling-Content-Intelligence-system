import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../context/AuthContext'
import { AppRoutes } from './index'

function mockLoginAs(role: 'admin' | 'coordinator' | 'instructor' | 'learner') {
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
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          user_id: 'u-1',
          tenant_id: 'tenant-1',
          email: 'u***@example.com',
          role,
          status: 'authenticated',
          session_id: 'session-1',
        }),
      }
    }),
  )
}

describe('Protected routes', () => {
  it('redirects unauthenticated users to login', async () => {
    render(
      <MemoryRouter initialEntries={['/reports']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    )

    expect(await screen.findByText('Sign in to continue')).toBeInTheDocument()
  })

  it('learner is blocked from reports route', async () => {
    mockLoginAs('learner')
    render(
      <MemoryRouter initialEntries={['/login']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    )
    fireEvent.change(screen.getByLabelText('Tenant ID'), { target: { value: 'tenant-1' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'learner@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'Password123!' } })
    fireEvent.click(screen.getByText('Sign In'))

    await waitFor(() => expect(screen.getByText('Workspace')).toBeInTheDocument())
    expect(screen.queryByText('Reports')).not.toBeInTheDocument()
    expect(screen.queryByText('Generate report')).not.toBeInTheDocument()
  })

  it('admin can open admin route', async () => {
    mockLoginAs('admin')
    render(
      <MemoryRouter initialEntries={['/login']}>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </MemoryRouter>,
    )
    fireEvent.change(screen.getByLabelText('Tenant ID'), { target: { value: 'tenant-1' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'admin@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'Password123!' } })
    fireEvent.click(screen.getByText('Sign In'))

    await waitFor(() => expect(screen.getByText('Workspace')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Admin'))
    expect(await screen.findByText('Create room')).toBeInTheDocument()
  })
})
